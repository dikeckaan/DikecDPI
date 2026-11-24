//go:build !windows

package main

import (
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// pinnedConnections counts TCP connections that other processes currently
// hold to our listen address. (netstat prints nothing for TCP when run from
// the app, so lsof is used; our own server-side sockets are skipped.)
func pinnedConnections(listen string) int {
	host, port, err := net.SplitHostPort(listen)
	if err != nil || port == "" {
		return 0
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	out, err := exec.Command("/usr/sbin/lsof", "-nP", "-iTCP@"+host+":"+port, "-sTCP:ESTABLISHED").
		Output()
	if err != nil && len(out) == 0 {
		return 0
	}
	self := strconv.Itoa(os.Getpid())
	n := 0
	for i, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if i == 0 || len(f) < 2 || f[1] == self {
			continue
		}
		n++
	}
	return n
}
