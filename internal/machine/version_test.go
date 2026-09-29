package machine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestVersionAtLeast(t *testing.T) {
	b, err := os.ReadFile("testdata/versions.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Actual, Minimum string
		Want            bool
	}
	if err = json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Actual+"_vs_"+c.Minimum, func(t *testing.T) {
			if got := VersionAtLeast(c.Actual, c.Minimum); got != c.Want {
				t.Fatalf("got %v, want %v", got, c.Want)
			}
		})
	}
	if !VersionAtLeast(Version, MinimumDeploymentVersion) {
		t.Fatal("current Agent must meet deployment minimum")
	}
}
