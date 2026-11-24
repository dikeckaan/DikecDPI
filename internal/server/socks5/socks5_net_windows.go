//go:build windows

package socks5

import (
	"os"
	"path/filepath"
)

var StateFile = filepath.Join(os.TempDir(), "spoofdpi.socks5.windows.state")
