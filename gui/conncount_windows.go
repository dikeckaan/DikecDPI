//go:build windows

package main

import (
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// pinnedConnections counts TCP connections from other processes to our
// listen address, using `netstat -ano` (columns: proto local foreign state pid).
func pinnedConnections(listen string) int {
	host, port, err := net.SplitHostPort(listen)
	if err != nil || port == "" {
		return 0
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	cmd := exec.Command("netstat", "-ano", "-p", "TCP")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	target := net.JoinHostPort(host, port)
	self := strconv.Itoa(os.Getpid())
	n := 0
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		// Count the client side: foreign == our address, owned by someone else.
		if len(f) >= 5 && f[2] == target && f[3] == "ESTABLISHED" && f[4] != self {
			n++
		}
	}
	return n
}
