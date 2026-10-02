package agent

import (
	"os"
	"path/filepath"
	"strings"
	"xingdu.app/xingdu/internal/protocol"
)

// Upstream executables remain unchanged. The service cgroup enforces the
// destination boundary for every packet, including reused UDP associations.
// This also disallows clients with private source addresses and local DNS;
// runtime configs use public DNS instead. AF_UNIX backend connections are not IP.
const officialDeniedNetworks = "0.0.0.0/8 10.0.0.0/8 100.64.0.0/10 127.0.0.0/8 169.254.0.0/16 172.16.0.0/12 192.168.0.0/16 168.63.129.16/32 224.0.0.0/4 240.0.0.0/4 ::/128 ::1/128 fc00::/7 fe80::/10 ff00::/8 ::ffff:0:0/96"

func officialRuntimeBinary(binary string) bool {
	name := filepath.Base(binary)
	return name == "sing-box-"+protocol.RuntimeVersion || name == "xray-"+protocol.XrayVersion
}
func (x *protocolExecutor) requiresEgressFilter(id string) bool {
	if x.trustTunnelUnit(id) {
		return true
	}
	unit, err := os.ReadFile(filepath.Join(x.unitDir, serviceName(x.localDeploymentID(id))))
	return err == nil && strings.Contains(string(unit), "# Xingdu official runtime egress guard\nIPAddressDeny="+officialDeniedNetworks+"\n")
}
