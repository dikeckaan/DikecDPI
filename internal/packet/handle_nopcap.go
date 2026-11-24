//go:build windows || ios

package packet

import (
	"errors"
	"net"
)

// Raw packet capture/injection (fake packets with a low TTL) needs libpcap/Npcap
// and cgo, which Windows release builds and iOS do not have. Profiles with fake
// packets report this error instead; everything else works.
var errNoPcap = errors.New("fake packets are not supported on this platform")

func NewHandle(_ *net.Interface) (Handle, error) {
	return nil, errNoPcap
}
