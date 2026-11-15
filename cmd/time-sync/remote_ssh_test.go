package main

import (
    "errors"
    "fmt"
    "strings"
    "testing"
    "time"
)

func TestSSHTargetValidation(t *testing.T) {
    for _, target := range []string{"root@iphone.local", "root@192.168.1.42"} {
        if err := validateSSHTarget(target); err != nil { t.Errorf("rejected %q: %v", target, err) }
    }
    for _, target := range []string{"", "user@iphone.local", "root@-invalid", "root@host;touch /tmp/x", "root@host -o ProxyCommand=evil", "root@[::1]", "-oProxyCommand=evil"} {
        if err := validateSSHTarget(target); err == nil { t.Errorf("accepted %q", target) }
    }
}

func TestRemoteClockUsesOnlyFixedCommands(t *testing.T) {
    var calls []string
    run := func(cmd string) (string, error) {
        calls = append(calls, cmd)
        if cmd == "date -u +%s" { return fmt.Sprint(time.Now().Unix()) + "\n", nil }
        return "ok", nil
    }
    if err := setRemoteClock(30*time.Millisecond, 2*time.Minute, run); err != nil { t.Fatal(err) }
    if len(calls) != 2 || calls[0] != "date -u +%s" || !strings.HasPrefix(calls[1], "date -u ") || len(calls[1]) != len("date -u 111512302026.13") {
        t.Fatalf("unexpected remote commands: %v", calls)
    }
    if strings.ContainsAny(strings.TrimPrefix(calls[1], "date -u "), " ;${}") { t.Fatalf("unsafe command: %s", calls[1]) }
}

func TestRemoteClockRefusesUnsafeDifference(t *testing.T) {
    calls := 0
    run := func(cmd string) (string, error) {
        calls++
        if cmd != "date -u +%s" { t.Fatalf("tried to set clock: %s", cmd) }
        return fmt.Sprint(time.Now().Add(-2*time.Hour).Unix()), nil
    }
    if err := setRemoteClock(0, time.Minute, run); err == nil || calls != 1 { t.Fatalf("err=%v calls=%d", err, calls) }
}

func TestRemoteClockFailsClosedOnReadFailure(t *testing.T) {
    calls := 0
    run := func(_ string) (string, error) { calls++; return "", errors.New("SSH host key not trusted") }
    if err := setRemoteClock(0, time.Minute, run); err == nil || calls != 1 { t.Fatalf("err=%v calls=%d", err, calls) }
}

func TestRemoteClockRejectsInvalidTimestamp(t *testing.T) {
    calls := 0
    run := func(_ string) (string, error) { calls++; return "invalid date", nil }
    if err := setRemoteClock(0, time.Minute, run); err == nil || calls != 1 { t.Fatalf("err=%v calls=%d", err, calls) }
}
