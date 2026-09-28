package machine

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"time"
)

const Version = "0.4.0-dev"

type Metrics struct {
	Hostname        string  `json:"hostname"`
	OS              string  `json:"os"`
	Arch            string  `json:"arch"`
	Version         string  `json:"version"`
	Uptime          float64 `json:"uptime_seconds"`
	MemoryTotal     uint64  `json:"memory_total_bytes"`
	MemoryAvailable uint64  `json:"memory_available_bytes"`
	Load1           float64 `json:"load_1"`
	CPUs            int     `json:"cpus"`
}

func (m Metrics) Validate() error {
	if len(m.Hostname) > 253 || len(m.OS) > 64 || len(m.Arch) > 32 || len(m.Version) > 64 || m.Uptime < 0 || m.Load1 < 0 || m.CPUs < 1 || m.CPUs > 65536 || m.MemoryAvailable > m.MemoryTotal {
		return errors.New("invalid metrics")
	}
	return nil
}

type Target struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
	User    string `json:"user"`
}
type Secret struct {
	Target          Target `json:"target"`
	Method          string `json:"method"`
	Password        string `json:"password,omitempty"`
	PrivateKey      string `json:"private_key,omitempty"`
	Passphrase      string `json:"passphrase,omitempty"`
	Fingerprint     string `json:"fingerprint"`
	EnrollmentToken string `json:"enrollment_token,omitempty"`
	Mode            string `json:"mode"`
}
type Job struct {
	ID         string     `json:"id"`
	HostID     string     `json:"host_id"`
	OrgID      string     `json:"-"`
	UserID     string     `json:"-"`
	Mode       string     `json:"mode"`
	State      string     `json:"state"`
	Result     string     `json:"result"`
	CreatedAt  time.Time  `json:"created_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Encrypted  []byte     `json:"-"`
	Lease      string     `json:"-"`
}
type AgentInfo struct {
	Mode       string     `json:"mode"`
	EnrolledAt time.Time  `json:"enrolled_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	Metrics    Metrics    `json:"metrics"`
}
type SavedCredential struct {
	Method      string    `json:"method"`
	Fingerprint string    `json:"fingerprint"`
	SavedAt     time.Time `json:"saved_at"`
	Encrypted   []byte    `json:"-"`
}

func Token() string            { b := make([]byte, 32); rand.Read(b); return hex.EncodeToString(b) }
func Hash(t string) string     { b := sha256.Sum256([]byte(t)); return hex.EncodeToString(b[:]) }
func ValidToken(t string) bool { b, e := hex.DecodeString(t); return e == nil && len(b) == 32 }
func ValidMode(m string) bool  { return m == "monitor" || m == "manage" }
func Origin(s string) error {
	u, e := url.Parse(s)
	if e != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("invalid control-plane origin")
	}
	if u.Scheme == "https" {
		return nil
	}
	if u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1") {
		return nil
	}
	return errors.New("control plane requires HTTPS")
}
func AAD(org, host, id string) string {
	return strings.Join([]string{"xingdu-ssh-v1", org, host, id}, ":")
}
