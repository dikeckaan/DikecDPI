//go:build windows

package netutil

import "syscall"

// IPV6_UNICAST_HOPS from ws2ipdef.h; not exported by package syscall on Windows.
const ipv6UnicastHops = 4

func setsockoptInt(fd uintptr, level, opt, value int) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), level, opt, value)
}
