package protocol

// RuntimeVersion is pinned intentionally; upgrades require configuration and
// client-handshake acceptance checks, not an unreviewed "latest" download.
const RuntimeVersion = "1.14.2"
const HardenedRuntimeVersion = "1.14.2-xingdu.1"

// Historical patched executable hashes remain available for older Agents.
var HardenedRuntimeSHA256 = map[string]string{
	"amd64": "350177a4aee448381690ca3bcefc38009d42370fc915f577ad5d7ed5b9c25250",
	"arm64": "d755d2ce931a888d14e94f3840ff0fee8b34c78d02c657cf1ec0026a56c4fbda",
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

// New Agents use the pinned official Xray release; historical builds are retained.
const XrayVersion = "26.9.9"
const HardenedXrayVersion = "26.3.27-xingdu.1"

var HardenedXraySHA256 = map[string]string{
	"amd64": "b9c16509df923e99ca43a2258d18156943e960ae21b983b85ca8c2af382e2f7f",
	"arm64": "e04e3d874502e5cafcbbe13216a3024d057bcd7d767ea82949f8931c6ca2d5a3",
}
var XrayArchiveSHA256 = map[string]string{
	"amd64": "23cd9af937744d97776ee35ecad4972cf4b2109d1e0fe6be9930467608f7c8ae",
	"arm64": "4d30283ae614e3057f730f67cd088a42be6fdf91f8639d82cb69e48cde80413c",
}

// Keep the original endpoint available for Agents whose compiled-in digest
// predates the hardened runtime. New Agents use an explicit family URL.
const LegacyRuntimeVersion = "1.14.2"

var LegacyRuntimeSHA256 = map[string]string{
	"amd64": "fc9c6e6ab345f045b16a0ed10d1ff28d68e8e56e7749fca30738d1406e98d7b8",
	"arm64": "b8610f45abb7e967e195264383f5cbd20fba7821a3c37e3a8c4c5ab6cad28eac",
}

// Official executables are extracted unchanged from verified upstream archives.
var RuntimeSHA256 = map[string]string{
	"amd64": "fc9c6e6ab345f045b16a0ed10d1ff28d68e8e56e7749fca30738d1406e98d7b8",
	"arm64": "b8610f45abb7e967e195264383f5cbd20fba7821a3c37e3a8c4c5ab6cad28eac",
}
var XraySHA256 = map[string]string{
	"amd64": "c4ae6798c38e0e5343b192406746333cd0ba7ff3eb984f8c4b9939dcb68c3f8a",
	"arm64": "c1defe42b6db958a97c5e049a02a00a4baaedaca7b51c1c229f0830e288acef5",
}
var OfficialXrayArchiveSHA256 = map[string]string{
	"amd64": "1eb9175d0f0a8f8149c9230a7fc5ae66ce332ed20a53155ce61fe62e3f58b7df",
	"arm64": "3e38d72dfc5eb65c91df0e5583e9b6676c32232041da47de6ae73946b526d66c",
}
