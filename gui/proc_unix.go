//go:build !windows

package main

import (
	"errors"
	"syscall"
)

// processAlive works for root-owned processes too: EPERM means "exists".
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// detachedProcAttr puts a child in its own process group so it survives
// this app quitting.
func detachedProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}
