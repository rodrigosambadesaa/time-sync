//go:build android

package main

import (
    "errors"
    "fmt"
    "os"
    "syscall"
    "time"
)

// setClock requires a root shell with CAP_SYS_TIME and SELinux permission.
// On Android, root UID alone does not guarantee that the kernel permits it.
func setClock(offset time.Duration) error {
    if os.Geteuid() != 0 {
        return errors.New("Android system clock update requires a root shell (su) and CAP_SYS_TIME")
    }
    corrected := time.Now().UTC().Add(offset)
    tv := syscall.NsecToTimeval(corrected.UnixNano())
    if err := syscall.Settimeofday(&tv); err != nil {
        return fmt.Errorf("Android settimeofday rejected: %w (check CAP_SYS_TIME, SELinux and automatic time settings)", err)
    }
    return nil
}
