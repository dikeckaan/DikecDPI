//go:build !windows

package netutil

import "syscall"

const ipv6UnicastHops = syscall.IPV6_UNICAST_HOPS

func setsockoptInt(fd uintptr, level, opt, value int) error {
	return syscall.SetsockoptInt(int(fd), level, opt, value)
}
