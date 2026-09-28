#!/bin/sh
set -eu
# Private cgroup namespace only; never mount the host's cgroup tree here.
mount -o remount,rw /sys/fs/cgroup
mkdir -p /run/sshd
ssh-keygen -A >/dev/null
if command -v update-ca-certificates >/dev/null; then
    cp /lab-ca/ca.crt /usr/local/share/ca-certificates/xingdu-lab.crt
    update-ca-certificates >/dev/null
else
    cp /lab-ca/ca.crt /etc/pki/ca-trust/source/anchors/xingdu-lab.crt
    update-ca-trust
fi
exec /sbin/init
