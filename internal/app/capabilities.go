//go:build !linux

package app

// dropCapabilites is a no-op in any platform other than linux
// For its behaviour in linux checl capabilties_linux.go
func dropCapabilites() error {
	return nil
}
