//go:build windows

package mobile

import (
	"errors"

	"golang.zx2c4.com/wireguard/tun"
)

// The mobile core is for Android and iOS; Windows uses the desktop app.
func createTUNFromFD(int, int) (tun.Device, error) {
	return nil, errors.New("mobile core is not supported on Windows")
}
