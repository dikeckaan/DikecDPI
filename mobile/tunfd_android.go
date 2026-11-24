//go:build android

package mobile

import "golang.zx2c4.com/wireguard/tun"

// Android forbids the netlink monitoring that CreateTUNFromFile sets up, so
// use the unmonitored variant (as wireguard-android does). The MTU is set by
// VpnService.Builder.
func createTUNFromFD(fd int, _ int) (tun.Device, error) {
	dev, _, err := tun.CreateUnmonitoredTUNFromFD(fd)
	return dev, err
}
