//go:build linux && !android

package main

func statusPath() string { return "/var/lib/time-sync/status.json" }
