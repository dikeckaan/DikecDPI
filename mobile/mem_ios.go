//go:build ios

package mobile

import "runtime/debug"

// iOS kills a Network Extension that uses more than ~50 MB. Keep the Go heap
// well below that and collect more eagerly.
func init() {
	debug.SetMemoryLimit(28 << 20)
	debug.SetGCPercent(40)
}
