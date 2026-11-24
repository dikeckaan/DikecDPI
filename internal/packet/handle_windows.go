//go:build windows

package packet

import (
	"errors"
	"net"
)

// Raw packet capture/injection (fake packets with a low TTL) needs Npcap and
// cgo on Windows, which the release builds do not use. Profiles with fake
// packets report this error instead; everything else works.
var errNoPcapOnWindows = errors.New("fake packets are not supported on Windows yet")

func NewHandle(_ *net.Interface) (Handle, error) {
	return nil, errNoPcapOnWindows
}
