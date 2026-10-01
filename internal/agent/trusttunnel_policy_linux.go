//go:build linux

package agent

import (
	"golang.org/x/sys/unix"
	"runtime"
	"unsafe"
)

// Query the exact service cgroup, rather than assuming IPAddressDeny was
// enforced because systemd accepted the unit on a kernel without BPF support.
func hasEgressFilter(path string) bool {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return false
	}
	defer unix.Close(fd)
	// Keep the complete Linux bpf_attr.query output layout, including revision.
	// Kernels write the revision even for a count-only query.
	// https://github.com/torvalds/linux/blob/master/include/uapi/linux/bpf.h
	query := struct {
		TargetFD        uint32
		AttachType      uint32
		QueryFlags      uint32
		AttachFlags     uint32
		ProgIDs         uint64
		ProgCount       uint32
		_               uint32
		ProgAttachFlags uint64
		LinkIDs         uint64
		LinkAttachFlags uint64
		Revision        uint64
	}{TargetFD: uint32(fd), AttachType: unix.BPF_CGROUP_INET_EGRESS}
	_, _, errno := unix.Syscall(unix.SYS_BPF, unix.BPF_PROG_QUERY, uintptr(unsafe.Pointer(&query)), unsafe.Sizeof(query))
	runtime.KeepAlive(&query)
	return (errno == 0 || errno == unix.ENOSPC) && query.ProgCount > 0
}
