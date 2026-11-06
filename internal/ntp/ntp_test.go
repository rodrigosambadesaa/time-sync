package ntp

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestTimestampRoundTrip(t *testing.T) {
	for _, date := range []time.Time{
		time.Date(2026, 11, 6, 15, 0, 0, 123456789, time.UTC),
		time.Date(2040, 1, 1, 0, 0, 0, 999999999, time.UTC),
	} {
		var buf [8]byte
		putTimestamp(buf[:], date)
		got := parseTimestamp(buf[:], date)
		if d := got.Sub(date); d > time.Nanosecond || d < -time.Nanosecond {
			t.Fatalf("timestamp delta=%s", d)
		}
	}
}
func TestRejectOneServer(t *testing.T) {
	if _, _, e := Consensus([]Sample{{Server: "a"}}); e == nil {
		t.Fatal("accepted one server")
	}
}
func TestConsensusExcludesOutlier(t *testing.T) {
	samples := []Sample{{Server: "a", Offset: 10 * time.Millisecond}, {Server: "b", Offset: 12 * time.Millisecond}, {Server: "c", Offset: 2 * time.Second}}
	got, kept, err := Consensus(samples)
	if err != nil || got != 11*time.Millisecond || len(kept) != 2 {
		t.Fatalf("offset=%s kept=%v err=%v", got, kept, err)
	}
}
func TestConsensusRejectsInconsistent(t *testing.T) {
	_, _, err := Consensus([]Sample{{Server: "a", Offset: 0}, {Server: "b", Offset: time.Second}})
	if err == nil {
		t.Fatal("accepted inconsistent sources")
	}
}
func TestNtpEra2036(t *testing.T) {
	ref := time.Date(2036, 3, 1, 0, 0, 0, 0, time.UTC)
	var buf [8]byte
	putTimestamp(buf[:], ref)
	if binary.BigEndian.Uint32(buf[:4]) >= 1<<31 {
		t.Fatal("expected rollover")
	}
	if got := parseTimestamp(buf[:], ref); !got.Equal(ref) {
		t.Fatalf("era: got %s", got)
	}
}
