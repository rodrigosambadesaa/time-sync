package ntp

import (
	"context"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"
)

// Local NTP responder tests packet parsing and the real UDP exchange offline.
func localNTPResponder(t *testing.T, change func([]byte)) string {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	go func() {
		var request [512]byte
		n, addr, e := conn.ReadFrom(request[:])
		if e != nil || n < 48 {
			return
		}
		var response [48]byte
		response[0] = 0x24 // NTPv4 server mode
		response[1] = 2
		binary.BigEndian.PutUint32(response[8:12], 1<<16)
		copy(response[24:32], request[40:48])
		now := time.Now().UTC()
		putTimestamp(response[32:40], now)
		putTimestamp(response[40:48], now)
		if change != nil {
			change(response[:])
		}
		_, _ = conn.WriteTo(response[:], addr)
	}()
	return conn.LocalAddr().String()
}
func TestQueryValidLocalUDP(t *testing.T) {
	addr := localNTPResponder(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s, err := queryAddr(ctx, "local", addr)
	if err != nil {
		t.Fatal(err)
	}
	if s.Server != "local" || s.Stratum != 2 || s.RTT < 0 {
		t.Fatalf("unexpected sample: %+v", s)
	}
}
func TestQueryRejectsReplay(t *testing.T) {
	addr := localNTPResponder(t, func(p []byte) { p[24] ^= 0x01 })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := queryAddr(ctx, "local", addr)
	if err == nil || !strings.Contains(err.Error(), "origin token mismatch") {
		t.Fatalf("err=%v", err)
	}
}
func TestQueryRejectsUnreliableServer(t *testing.T) {
	addr := localNTPResponder(t, func(p []byte) { binary.BigEndian.PutUint32(p[8:12], 3<<16) })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := queryAddr(ctx, "local", addr)
	if err == nil || !strings.Contains(err.Error(), "dispersion") {
		t.Fatalf("err=%v", err)
	}
}
