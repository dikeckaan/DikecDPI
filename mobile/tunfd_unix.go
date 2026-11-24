//go:build !windows && !android

package mobile

import (
	"os"

	"golang.zx2c4.com/wireguard/tun"
)

func createTUNFromFD(fd int, mtu int) (tun.Device, error) {
	if mtu <= 0 {
		mtu = 1500
	}
	return tun.CreateTUNFromFile(os.NewFile(uintptr(fd), "tun"), mtu)
}
