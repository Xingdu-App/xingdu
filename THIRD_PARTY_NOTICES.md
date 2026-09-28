# Third-party runtime notices

Xingdu's original source code is licensed under [MIT](LICENSE). Protocol deployments run the unmodified **sing-box 1.14.2** executable as a separate systemd process. sing-box is a third-party component, is not relicensed under Xingdu's MIT license, and is not affiliated with Xingdu.

- [Upstream release and binary assets](https://github.com/SagerNet/sing-box/releases/tag/v1.14.2)
- [Versioned source code and build configuration](https://github.com/SagerNet/sing-box/tree/v1.14.2)
- [Upstream license](https://github.com/SagerNet/sing-box/blob/v1.14.2/LICENSE): GNU GPL version 3 or later, with the upstream naming/association notice.
- [GNU GPL version 3 full text](https://www.gnu.org/licenses/gpl-3.0.html)

`make runtimes` downloads the official archives, verifies pinned archive and executable SHA-256 hashes, and preserves the upstream `LICENSE` as `bin/agents/sing-box-LICENSE`. Docker builds include the same license at `/opt/xingdu/agents/sing-box-LICENSE`; managed machines receive the embedded notice at `/usr/local/lib/xingdu/sing-box-LICENSE`. The pinned versions and hashes are maintained in `internal/protocol/runtime_manifest.go`; no tenant can select a binary URL or a runtime version.

The selected protocols do not use the optional Naive/Chromium library included in the upstream archives; Xingdu does not copy or distribute that library. Client applications such as Stash, Surge, Loon and Shadowrocket are separate products; their names do not imply endorsement or verified compatibility with every protocol.

When redistributing runtime binaries, retain the upstream notices and meet their source-distribution obligations. This repository does not claim that an upstream download link alone fulfills every redistribution scenario. Release packaging and corresponding-source delivery must be reviewed before publishing distributable binary releases.
