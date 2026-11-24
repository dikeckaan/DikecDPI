//go:build windows

package http

import (
	"os"
	"path/filepath"
)

// StateFile records the system proxy changes so they can be undone, also
// after a crash.
var StateFile = filepath.Join(os.TempDir(), "spoofdpi.http.windows.state")
