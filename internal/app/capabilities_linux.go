package app

import (
	"fmt"

	"kernel.org/pub/linux/libs/security/libcap/cap"
)

// dropLinuxCapabilities removes all Linux capabilities unnecessary for normal
// operation. While this improves security a bit, it can be a major source of
// permission denied errors.
func dropCapabilites() error {
	currentCaps := cap.GetProc()

	// we need to preserve net_bind_service otherwise we will never be able to bind
	// to a privileged port (<1024), not even with root.
	isBindPermited, err := currentCaps.GetFlag(cap.Permitted, cap.NET_BIND_SERVICE)
	if err != nil {
		return fmt.Errorf("failed to query for net_bind_service capability: %w", err)
	}

	newCaps := cap.NewSet()

	if isBindPermited {
		// these can't fail as the new set is yet to be associated with the process.
		_ = newCaps.SetFlag(cap.Permitted, true, cap.NET_BIND_SERVICE)
		_ = newCaps.SetFlag(cap.Effective, true, cap.NET_BIND_SERVICE)
	}

	if err = newCaps.SetProc(); err != nil {
		return fmt.Errorf("failed to set process capabilites: %w", err)
	}

	return nil
}
