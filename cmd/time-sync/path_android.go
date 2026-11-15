//go:build android

package main

// Root-owned application data, not the untrusted shared external storage.
func statusPath() string { return "/data/adb/time-sync/status.json" }
