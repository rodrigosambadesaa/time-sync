// Package ntp implements a small, defensive SNTP client (RFC 5905).
// NTP packets are NOT authenticated; use only on trusted networks.
package ntp

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sort"
	"time"
)

const epoch = int64(2208988800)

// Sample is a single four-timestamp exchange with an NTP server.
type Sample struct {
	Server  string        `json:"server"`
	Offset  time.Duration `json:"offset_ns"`
	RTT     time.Duration `json:"rtt_ns"`
	Stratum uint8         `json:"stratum"`
}

// Query obtains one time sample, validating the server response and replay token.
func Query(ctx context.Context, host string) (Sample, error) {
	result := Sample{Server: host}
	conn, err := (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "udp", net.JoinHostPort(host, "123"))
	if err != nil {
		return result, err
	}
	defer conn.Close()
	deadline := time.Now().Add(2 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return result, err
	}
	var request [48]byte
	request[0] = 0x23 // version 4, client mode 3
	t1 := time.Now().UTC()
	putTimestamp(request[40:48], t1)
	if _, err := conn.Write(request[:]); err != nil {
		return result, err
	}
	var response [512]byte
	n, err := conn.Read(response[:])
	t4 := time.Now().UTC()
	if err != nil {
		return result, err
	}
	if n < 48 {
		return result, errors.New("truncated NTP packet")
	}
	p := response[:n]
	if p[0]&7 != 4 || p[0]>>6 == 3 || p[1] == 0 || p[1] > 15 {
		return result, errors.New("invalid NTP mode, leap state or stratum")
	}
	if !bytes.Equal(p[24:32], request[40:48]) {
		return result, errors.New("NTP origin token mismatch (spoof or replay)")
	}
	if bytes.Equal(p[32:40], make([]byte, 8)) || bytes.Equal(p[40:48], make([]byte, 8)) {
		return result, errors.New("missing server timestamps")
	}
	// The server's reported dispersion (16.16 fixed point) must be plausible.
	if binary.BigEndian.Uint32(p[8:12]) > 2<<16 {
		return result, errors.New("unreliable NTP root dispersion")
	}
	t2 := parseTimestamp(p[32:40], t1)
	t3 := parseTimestamp(p[40:48], t1)
	if t3.Before(t2.Add(-10*time.Millisecond)) || t3.Sub(t2) > time.Second {
		return result, errors.New("invalid server processing interval")
	}
	rtt := t4.Sub(t1) - t3.Sub(t2)
	if rtt < -10*time.Millisecond || rtt > 600*time.Millisecond {
		return result, fmt.Errorf("unacceptable round-trip delay: %s", rtt)
	}
	if rtt < 0 {
		rtt = 0
	} // Small negative values are possible due to quantization.
	result.Offset = (t2.Sub(t1) + t3.Sub(t4)) / 2
	result.RTT = rtt
	result.Stratum = p[1]
	return result, nil
}

func putTimestamp(out []byte, t time.Time) {
	binary.BigEndian.PutUint32(out[0:4], uint32(t.Unix()+epoch))
	binary.BigEndian.PutUint32(out[4:8], uint32((uint64(t.Nanosecond())<<32)/1_000_000_000))
}

func parseTimestamp(raw []byte, reference time.Time) time.Time {
	seconds := int64(binary.BigEndian.Uint32(raw[:4]))
	referenceNTP := reference.Unix() + epoch
	// Select the NTP era nearest to the local clock, including the 2036 wrap.
	era := (referenceNTP - seconds + (1 << 31)) >> 32
	unix := seconds + (era << 32) - epoch
	nanos := int64(float64(binary.BigEndian.Uint32(raw[4:8])) * 1e9 / 4294967296.0)
	return time.Unix(unix, nanos).UTC()
}

// Consensus returns the median offset of the largest tight cluster of samples.
// It refuses to update the clock unless two distinct servers agree.
func Consensus(samples []Sample) (time.Duration, []Sample, error) {
	if len(samples) < 2 {
		return 0, nil, errors.New("at least two NTP servers required")
	}
	ordered := append([]Sample(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Offset < ordered[j].Offset })
	bestStart, bestEnd := 0, 0
	for i := range ordered {
		j := i + 1
		for j < len(ordered) && ordered[j].Offset-ordered[i].Offset <= 150*time.Millisecond {
			j++
		}
		if j-i > bestEnd-bestStart {
			bestStart, bestEnd = i, j
		}
	}
	cluster := ordered[bestStart:bestEnd]
	distinct := map[string]struct{}{}
	for _, s := range cluster {
		distinct[s.Server] = struct{}{}
	}
	if len(distinct) < 2 {
		return 0, nil, errors.New("NTP servers do not agree within 150 ms")
	}
	mid := len(cluster) / 2
	if len(cluster)%2 == 1 {
		return cluster[mid].Offset, cluster, nil
	}
	return cluster[mid-1].Offset/2 + cluster[mid].Offset/2, cluster, nil
}
