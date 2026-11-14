//go:build linux || darwin

package main

import (
	"fmt"
	"syscall"
	"time"
)

// setClock changes CLOCK_REALTIME with native Unix settimeofday. Root (or on
// Linux CAP_SYS_TIME) is normally required. No clock adjustment is attempted
// unless the NTP consensus and safety checks have succeeded.
func setClock(offset time.Duration) error {
	t := time.Now().UTC().Add(offset)
	tv := syscall.NsecToTimeval(t.UnixNano())
	if err := syscall.Settimeofday(&tv); err != nil {
		return fmt.Errorf("settimeofday: %w (administrator privilege and OS time-service settings may be required)", err)
	}
	return nil
}
