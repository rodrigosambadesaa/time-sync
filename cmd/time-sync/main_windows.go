package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/rodrigosambadesaa/time-sync/internal/ntp"
)

var setSystemTime = syscall.NewLazyDLL("kernel32.dll").NewProc("SetSystemTime")

type systemTime struct{ Year, Month, DayOfWeek, Day, Hour, Minute, Second, Milliseconds uint16 }

type result struct {
	AtUTC              time.Time    `json:"at_utc"`
	OffsetMilliseconds float64      `json:"offset_ms"`
	Applied            bool         `json:"applied"`
	Servers            []ntp.Sample `json:"servers"`
	Error              string       `json:"error,omitempty"`
}

func writeStatus(r result) {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = os.TempDir()
	}
	path := filepath.Join(base, "TimeSync", "status.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Fprintln(os.Stderr, "status:", err)
		return
	}
	data, _ := json.MarshalIndent(r, "", "  ")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		fmt.Fprintln(os.Stderr, "status:", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path) // Windows Rename cannot overwrite an existing destination.
		if e := os.Rename(tmp, path); e != nil {
			fmt.Fprintln(os.Stderr, "status:", e)
		}
	}
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

func main() {
	mode := flag.String("mode", "sync", "sync or check (check never sets system time)")
	peers := flag.String("servers", "time.cloudflare.com,time.windows.com,0.es.pool.ntp.org,1.es.pool.ntp.org", "comma-separated NTP hosts")
	maxStep := flag.Duration("max-step", 10*time.Minute, "refuse clock corrections larger than this")
	flag.Parse()
	if *mode != "sync" && *mode != "check" {
		fmt.Fprintln(os.Stderr, "-mode must be sync or check")
		os.Exit(2)
	}
	servers := strings.Split(*peers, ",")
	if len(servers) < 2 || len(servers) > 8 {
		fmt.Fprintln(os.Stderr, "provide 2-8 NTP servers")
		os.Exit(2)
	}
	seen := map[string]bool{}
	for i, s := range servers {
		servers[i] = strings.TrimSpace(s)
		if servers[i] == "" || seen[servers[i]] {
			fmt.Fprintln(os.Stderr, "NTP servers must be unique and nonempty")
			os.Exit(2)
		}
		seen[servers[i]] = true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type reply struct {
		s ntp.Sample
		e error
	}
	ch := make(chan reply, len(servers))
	var wg sync.WaitGroup
	for _, server := range servers {
		wg.Add(1)
		go func(server string) { defer wg.Done(); s, e := ntp.Query(ctx, server); ch <- reply{s, e} }(server)
	}
	wg.Wait()
	close(ch)
	status := result{AtUTC: time.Now().UTC()}
	for r := range ch {
		if r.e != nil {
			fmt.Fprintf(os.Stderr, "NTP %s: %v\n", r.s.Server, r.e)
			continue
		}
		status.Servers = append(status.Servers, r.s)
	}
	offset, agreed, err := ntp.Consensus(status.Servers)
	if err == nil && (offset > *maxStep || offset < -*maxStep) {
		err = fmt.Errorf("unsafe offset %s exceeds limit %s", offset, *maxStep)
	}
	status.OffsetMilliseconds = float64(offset) / float64(time.Millisecond)
	if err == nil && *mode == "sync" {
		err = setClock(offset)
		status.Applied = err == nil
	}
	if err != nil {
		status.Error = err.Error()
	}
	if *mode == "sync" {
		writeStatus(status)
	}
	output, _ := json.MarshalIndent(status, "", "  ")
	fmt.Println(string(output))
	if err != nil {
		fmt.Fprintln(os.Stderr, "time synchronization failed:", err)
		os.Exit(1)
	}
	fmt.Printf("Agreed NTP servers: %d; source validation complete.\n", len(agreed))
}
