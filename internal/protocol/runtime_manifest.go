package protocol

// RuntimeVersion is pinned intentionally; upgrades require configuration and
// client-handshake acceptance checks, not an unreviewed "latest" download.
const RuntimeVersion = "1.14.2"

// RuntimeSHA256 verifies the extracted executable again on each managed machine.
var RuntimeSHA256 = map[string]string{
	"amd64": "fc9c6e6ab345f045b16a0ed10d1ff28d68e8e56e7749fca30738d1406e98d7b8",
	"arm64": "b8610f45abb7e967e195264383f5cbd20fba7821a3c37e3a8c4c5ab6cad28eac",
}

// RuntimeArchiveSHA256 comes from the official release asset digests.
var RuntimeArchiveSHA256 = map[string]string{
	"amd64": "a684484d7477d1437282ee411f4d131d0340aaad60a7868841ebd5d87dd8a0c6",
	"arm64": "b43a1fb1bda131c6653576741ce527eb2bdeab7c9308ca90ee8b972abb7e4a7f",
}

// TrustTunnel is an independent, unmodified upstream endpoint (HTTP/2 only).
const TrustTunnelVersion = "1.1.0"

var TrustTunnelSHA256 = map[string]string{
	"amd64": "9bb15f4e30e2ff196cf0aa387462c42387d9405ddfeb5caad133838560fa33cd",
	"arm64": "0aa249672b1aa3a6fa7eadfb72159b7d965f6f006681f97a2528eed9161541c3",
}
var TrustTunnelArchiveSHA256 = map[string]string{
	"amd64": "91c2ea3db7416a01b5258a4c047ec22890490bc55e1b194206031aa75144f0e7",
	"arm64": "c2aee17a1ced349283cba4775202e2baba053b8ea835d4cc23dc67d16c6b9686",
}
