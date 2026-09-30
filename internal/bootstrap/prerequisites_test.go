package bootstrap

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Execute the actual POSIX preflight with isolated executable fixtures: never
// invoke the developer machine's package manager or require root for this test.
func TestTarPrerequisiteRecovery(t *testing.T) {
	for _, tc := range []struct {
		name, manager                           string
		present, fail, noTar, noRoot, noSystemd bool
		want                                    int
	}{
		{name: "already installed", manager: "apt-get", present: true},
		{name: "apt", manager: "apt-get"}, {name: "dnf", manager: "dnf"},
		{name: "yum", manager: "yum"}, {name: "zypper", manager: "zypper"},
		{name: "unsupported", want: 70},
		{name: "repository failure", manager: "apt-get", fail: true, want: 75},
		{name: "dnf failure", manager: "dnf", fail: true, want: 75},
		{name: "missing after install", manager: "yum", noTar: true, want: 75},
		{name: "non root", manager: "dnf", noRoot: true, want: 73},
		{name: "no systemd", manager: "dnf", noSystemd: true, want: 72},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write := func(name, body string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink("/bin/sh", filepath.Join(dir, "sh")); err != nil {
				t.Fatal(err)
			}
			uid := "0"
			if tc.noRoot {
				uid = "1000"
			}
			write("id", "echo "+uid)
			if !tc.noSystemd {
				write("systemctl", "exit 0")
			}
			if tc.present {
				write("tar", "exit 0")
			}
			if tc.manager != "" {
				body := "printf '%s\\n' \"$*\" >> \"$PATH/calls\"\n"
				if tc.fail {
					body += "echo remote-secret >&2; exit 1\n"
				} else if !tc.noTar {
					body += "case \" $* \" in *' install '*) printf '#!/bin/sh\\nexit 0\\n' > \"$PATH/tar\"; /bin/chmod 755 \"$PATH/tar\";; esac\n"
				}
				write(tc.manager, body)
			}
			cmd := exec.Command("/bin/sh", "-c", installPreflight)
			cmd.Env = []string{"PATH=" + dir}
			output, err := cmd.CombinedOutput()
			status := 0
			if err != nil {
				e, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatal(err)
				}
				status = e.ExitCode()
			}
			if status != tc.want {
				t.Fatalf("exit %d, want %d: %s", status, tc.want, output)
			}
			if len(output) != 0 {
				t.Fatalf("package output leaked: %s", output)
			}
			calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
			if tc.present || tc.noRoot || tc.noSystemd {
				if len(calls) != 0 {
					t.Fatalf("unexpected mutation: %s", calls)
				}
			} else if tc.manager != "" && !tc.fail && !strings.Contains(string(calls), "install") {
				t.Fatal("package installation not attempted")
			}
			if tc.manager == "apt-get" && !tc.present && !tc.fail && strings.Count(string(calls), "\n") != 2 {
				t.Fatalf("apt should update then install: %s", calls)
			}
		})
	}
}
