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
