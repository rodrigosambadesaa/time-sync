package main

import (
	"context"
	"errors"
	"github.com/rodrigosambadesaa/time-sync/internal/ntp"
	"strings"
    "sync/atomic"
	"testing"
	"time"
)

func fakeQuery(offsets map[string]time.Duration) queryFunc {
	return func(_ context.Context, s string) (ntp.Sample, error) {
		off, ok := offsets[s]
		if !ok {
			return ntp.Sample{Server: s}, errors.New("offline")
		}
		return ntp.Sample{Server: s, Offset: off, Authenticated: true}, nil
	}
}
func TestCheckNeverSetsSystemClock(t *testing.T) {
	calls := 0
	status, n, err := synchronize(context.Background(), []string{"a", "b", "c"}, "check", 10*time.Minute, true, fakeQuery(map[string]time.Duration{"a": 10 * time.Millisecond, "b": 12 * time.Millisecond, "c": 2 * time.Second}), func(time.Duration) error { calls++; return nil })
	if err != nil || n != 2 || status.Applied || calls != 0 || status.OffsetMilliseconds != 11 {
		t.Fatalf("status=%+v n=%d calls=%d err=%v", status, n, calls, err)
	}
}
func TestSyncCallsClockOnlyAfterConsensus(t *testing.T) {
	calls := 0
	status, n, err := synchronize(context.Background(), []string{"a", "b"}, "sync", time.Minute, true, fakeQuery(map[string]time.Duration{"a": 20 * time.Millisecond, "b": 20 * time.Millisecond}), func(off time.Duration) error {
		calls++
		if off != 20*time.Millisecond {
			t.Errorf("offset=%s", off)
		}
		return nil
	})
	if err != nil || n != 2 || !status.Applied || calls != 1 {
		t.Fatalf("status=%+v n=%d calls=%d err=%v", status, n, calls, err)
	}
}
func TestSyncRejectsExcessiveOffset(t *testing.T) {
	calls := 0
	status, _, err := synchronize(context.Background(), []string{"a", "b"}, "sync", time.Second, true, fakeQuery(map[string]time.Duration{"a": 5 * time.Second, "b": 5 * time.Second}), func(time.Duration) error { calls++; return nil })
	if err == nil || calls != 0 || status.Applied || !strings.Contains(status.Error, "unsafe offset") {
		t.Fatalf("status=%+v calls=%d err=%v", status, calls, err)
	}
}
func TestSyncRejectsInconsistentServers(t *testing.T) {
	calls := 0
	status, _, err := synchronize(context.Background(), []string{"a", "b"}, "sync", time.Minute, true, fakeQuery(map[string]time.Duration{"a": 0, "b": time.Second}), func(time.Duration) error { calls++; return nil })
	if err == nil || calls != 0 || status.Applied {
		t.Fatalf("status=%+v calls=%d err=%v", status, calls, err)
	}
}
func TestClockErrorPropagates(t *testing.T) {
	denied := errors.New("permission denied")
	status, _, err := synchronize(context.Background(), []string{"a", "b"}, "sync", time.Minute, true, fakeQuery(map[string]time.Duration{"a": 0, "b": 0}), func(time.Duration) error { return denied })
	if !errors.Is(err, denied) || status.Applied || status.Error != "permission denied" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}
func TestServerValidation(t *testing.T) {
	for _, v := range []string{"a", "a,a", "a,", "a,a,b", "a,b,c,d,e,f,g,h,i"} {
		if _, err := validateServers(v); err == nil {
			t.Errorf("accepted %q", v)
		}
	}
	if got, err := validateServers(" a , b "); err != nil || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestSecureModeRejectsUnauthenticatedResponsesEvenWithConsensus(t *testing.T) {
    called := false
    untrusted := func(_ context.Context, host string) (ntp.Sample, error) {
        return ntp.Sample{Server: host, Offset: 5*time.Millisecond}, nil
    }
    status, _, err := synchronize(context.Background(), []string{"one", "two"}, "sync", time.Minute, true, untrusted,
        func(time.Duration) error { called = true; return nil })
    if err == nil || called || status.Applied || status.AuthenticatedServers != 0 {
        t.Fatalf("accepted unauthenticated NTP: result=%+v err=%v", status, err)
    }
}
func TestSecureModeRejectsMixedAuthenticatedResponses(t *testing.T) {
    called := false
    mixed := func(_ context.Context, host string) (ntp.Sample, error) {
        return ntp.Sample{Server: host, Offset: 5*time.Millisecond, Authenticated: host=="one"}, nil
    }
    status, _, err := synchronize(context.Background(), []string{"one", "two"}, "sync", time.Minute, true, mixed,
        func(time.Duration) error { called = true; return nil })
    if err == nil || called || status.Applied {
        t.Fatalf("accepted mixed NTP sources: result=%+v err=%v", status, err)
    }
}

func TestRetryAfterResumeRecoversWithoutLosingNTS(t *testing.T) {
    var requests atomic.Int32
    clockCalls := 0
    query := func(_ context.Context, host string) (ntp.Sample, error) {
        count := requests.Add(1)
        if count <= 2 {
            return ntp.Sample{Server: host}, errors.New("network unavailable during wake")
        }
        return ntp.Sample{Server: host, Offset: 6*time.Millisecond, Authenticated: true}, nil
    }
    status, n, err := synchronizeWithRetry(context.Background(), []string{"a", "b"}, "sync", time.Minute, true, 3*time.Second, query,
        func(time.Duration) error { clockCalls++; return nil })
    if err != nil || n != 2 || !status.Applied || clockCalls != 1 || requests.Load() != 4 {
        t.Fatalf("status=%+v n=%d calls=%d requests=%d err=%v", status, n, clockCalls, requests.Load(), err)
    }
}
func TestRetryRejectsUnauthenticatedResponses(t *testing.T) {
    clockCalls := 0
    query := func(_ context.Context, host string) (ntp.Sample, error) {
        return ntp.Sample{Server: host, Offset: 6*time.Millisecond, Authenticated: false}, nil
    }
    status, _, err := synchronizeWithRetry(context.Background(), []string{"a", "b"}, "sync", time.Minute, true, 5*time.Millisecond, query,
        func(time.Duration) error { clockCalls++; return nil })
    if err == nil || status.Applied || clockCalls != 0 { t.Fatalf("status=%+v err=%v", status, err) }
}
