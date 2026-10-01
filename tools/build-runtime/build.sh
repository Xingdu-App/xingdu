#!/bin/sh
# Reproducible server runtimes. Inputs and patches are fixed by this repository.
set -eu
family=$1
arch=$2
output=$3
case "$arch" in amd64|arm64) ;; *) exit 2;; esac
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
mkdir -p "$output"
output=$(CDPATH= cd -- "$output" && pwd)
case "$family" in
 xray)
  repo=XTLS/Xray-core
  commit=d2758a023cd7f4174a5a5fa4ff66e487d4342ba0
  digest=14fa566ee0a801d3d51144c67018b449f5dcf462ddfacadd032da069787e61f9
  ;;
 sing-box)
  repo=SagerNet/sing-box
  commit=af6e64c3b69e6132ebaee0e1a3d24e93903f6709
  digest=bbf37fe816fc18621bd0700b71e69dd2a92e3e499f3311d7dff73319999174cf
  ;;
 *) exit 2;;
esac
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM
archive="$output/$family-upstream-source.tar.gz"
if ! test -f "$archive"; then
 curl --fail --location --proto '=https' --tlsv1.2 --retry 2 --max-time 180 "https://codeload.github.com/$repo/tar.gz/$commit" -o "$work/source.tar.gz"
 printf '%s  %s\n' "$digest" "$work/source.tar.gz" | sha256sum -c -
 mv "$work/source.tar.gz" "$archive"
fi
printf '%s  %s\n' "$digest" "$archive" | sha256sum -c -
tar xzf "$archive" -C "$work" --strip-components=1
cd "$work"
patch -p1 < "$root/tools/runtime-patches/$family-udp-guard.patch"
export GOTOOLCHAIN=go1.26.1 CGO_ENABLED=0 GOOS=linux GOARCH="$arch"
if test "$family" = xray; then
 # This ALPN fix also makes the same artifact usable as a reference client.
 patch -p1 < "$root/tools/compatibility/xray-http1-alpn.patch"
 go build -trimpath -buildvcs=false -o "$output/$family-linux-$arch.tmp" ./main
else
 go build -trimpath -buildvcs=false -tags with_gvisor,with_quic,with_wireguard,with_utls -ldflags '-X github.com/sagernet/sing-box/constant.Version=1.14.2-xingdu.1 -checklinkname=0' -o "$output/$family-linux-$arch.tmp" ./cmd/sing-box
fi
mv "$output/$family-linux-$arch.tmp" "$output/$family-linux-$arch"
cp LICENSE "$output/$family-LICENSE"
# Publish the complete modified upstream tree with its module checksums.
# The versioned Xingdu repository provides this build recipe and exact patches.
source_tmp=$(mktemp "$output/$family-source.XXXXXX")
tar czf "$source_tmp" .
chmod 0644 "$source_tmp"
mv "$source_tmp" "$output/$family-source.tar.gz"
