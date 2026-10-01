//go:build !linux

package agent

func hasEgressFilter(string) bool { return false }
