package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
)

// TrustTunnelConfig is a single atomic, root-only configuration revision. The
// launcher passes the derived upstream files through anonymous descriptors.
type TrustTunnelConfig struct {
	Runtime  string                `json:"runtime"`
	Spec     Spec                  `json:"spec"`
	Inbounds []TrustTunnelListener `json:"inbounds"`
}
type TrustTunnelListener struct {
	Port int `json:"listen_port"`
}

func RenderTrustTunnel(s Spec) ([]byte, error) {
	if s.Protocol != "trusttunnel" {
		return nil, errors.New("invalid TrustTunnel protocol")
	}
	if err := ValidateSpec(s); err != nil {
		return nil, err
	}
	return json.Marshal(TrustTunnelConfig{Runtime: "trusttunnel", Spec: s, Inbounds: []TrustTunnelListener{{Port: s.Port}}})
}

// TrustTunnelFiles returns fixed-order files for child descriptors 3..7. No
// tenant value is used as a path or command. Validated credentials and DNS names
// contain no TOML metacharacters; PEM material goes into separate descriptors.
func TrustTunnelFiles(s Spec) ([][]byte, error) {
	if s.Protocol != "trusttunnel" {
		return nil, errors.New("invalid TrustTunnel protocol")
	}
	if err := ValidateSpec(s); err != nil {
		return nil, err
	}
	main := fmt.Sprintf(`listen_address = "0.0.0.0:%d"
ipv6_available = true
allow_private_network_connections = false
credentials_file = "/proc/self/fd/5"
[listen_protocols.http2]
[forward_protocol]
direct = {}
`, s.Port)
	hosts := fmt.Sprintf("[[main_hosts]]\nhostname = %q\ncert_chain_path = \"/proc/self/fd/6\"\nprivate_key_path = \"/proc/self/fd/7\"\n", s.ServerName)
	credentials := fmt.Sprintf("[[client]]\nusername = \"xingdu\"\npassword = %q\n", s.Credential)
	return [][]byte{[]byte(main), []byte(hosts), []byte(credentials), []byte(s.Certificate), []byte(s.PrivateKey)}, nil
}
