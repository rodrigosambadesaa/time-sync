package main

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

var setSystemTime = syscall.NewLazyDLL("kernel32.dll").NewProc("SetSystemTime")

type systemTime struct{ Year, Month, DayOfWeek, Day, Hour, Minute, Second, Milliseconds uint16 }

func statusPath() string {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "TimeSync", "status.json")
}
func setClock(offset time.Duration) error {
	t := time.Now().UTC().Add(offset)
	st := systemTime{uint16(t.Year()), uint16(t.Month()), 0, uint16(t.Day()), uint16(t.Hour()), uint16(t.Minute()), uint16(t.Second()), uint16(t.Nanosecond() / 1e6)}
	r, _, e := setSystemTime.Call(uintptr(unsafe.Pointer(&st)))
	if r == 0 {
		if e != nil && e != syscall.Errno(0) {
			return e
		}
		return errors.New("SetSystemTime failed: administrative privilege required")
	}
	return nil
}
