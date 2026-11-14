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
	"time"

	"github.com/rodrigosambadesaa/time-sync/internal/ntp"
)

type result struct {
	AtUTC time.Time `json:"at_utc"`
    Security string `json:"security"`
    AuthenticatedServers int `json:"authenticated_servers"`
	OffsetMilliseconds float64      `json:"offset_ms"`
	Applied            bool         `json:"applied"`
	Servers            []ntp.Sample `json:"servers"`
	Error              string       `json:"error,omitempty"`
}

type queryFunc func(context.Context, string) (ntp.Sample, error)

// synchronize accepts injected network and clock operations so the entire decision
// path can be tested on all platforms without network access or setting host time.
func synchronize(ctx context.Context, servers []string, mode string, maxStep time.Duration, requireAuth bool, query queryFunc, apply func(time.Duration) error) (result, int, error) {
	type reply struct {
		sample ntp.Sample
		err    error
	}
	ch := make(chan reply, len(servers))
	var wg sync.WaitGroup
	for _, server := range servers {
		wg.Add(1)
		go func(server string) { defer wg.Done(); s, e := query(ctx, server); ch <- reply{s, e} }(server)
	}
	wg.Wait()
	close(ch)
	status := result{AtUTC: time.Now().UTC(), Security: "unauthenticated"}
    if requireAuth { status.Security = "nts" }
	for r := range ch {
		if r.err != nil {
			fmt.Fprintf(os.Stderr, "NTP %s: %v\n", r.sample.Server, r.err)
			continue
		}
		if requireAuth && !r.sample.Authenticated {
            fmt.Fprintf(os.Stderr, "Rejected unauthenticated time sample from %s\n", r.sample.Server)
            continue
        }
        status.Servers = append(status.Servers, r.sample)
	}
	offset, agreed, err := ntp.Consensus(status.Servers)
    for _, s := range agreed { if s.Authenticated { status.AuthenticatedServers++ } }
    if err == nil && ctx.Err() != nil { err = ctx.Err() }
	if err == nil && (offset > maxStep || offset < -maxStep) {
		err = fmt.Errorf("unsafe offset %s exceeds limit %s", offset, maxStep)
	}
	status.OffsetMilliseconds = float64(offset) / float64(time.Millisecond)
	if err == nil && mode == "sync" {
		err = apply(offset)
		status.Applied = err == nil
	}
	if err != nil {
		status.Error = err.Error()
	}
	return status, len(agreed), err
}

func validateServers(value string) ([]string, error) {
	servers := strings.Split(value, ",")
	if len(servers) < 2 || len(servers) > 8 {
		return nil, errors.New("provide 2-8 NTP servers")
	}
	seen := map[string]bool{}
	for i, s := range servers {
		servers[i] = strings.TrimSpace(s)
		if servers[i] == "" || seen[servers[i]] {
			return nil, errors.New("NTP servers must be unique and nonempty")
		}
		seen[servers[i]] = true
	}
	return servers, nil
}

func writeStatus(r result) {
	path := statusPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		fmt.Fprintln(os.Stderr, "status:", err)
		return
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "status:", err)
		return
	}
	// Store a complete status snapshot; never leave partial JSON after a crash.
	f, err := os.CreateTemp(filepath.Dir(path), ".status-*.tmp")
	if err != nil {
		fmt.Fprintln(os.Stderr, "status:", err)
		return
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0644); err == nil {
		_, err = f.Write(data)
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "status:", err)
		return
	}
	if err = os.Rename(tmp, path); err != nil {
		// Windows cannot rename over an existing destination.
		if err = os.Remove(path); err == nil {
			err = os.Rename(tmp, path)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "status:", err)
		}
	}
}

func main() {
	mode := flag.String("mode", "sync", "sync or check (check never sets system time)")
	peers := flag.String("servers", "", "comma-separated time server hosts (defaults depend on -auth)")
    auth := flag.String("auth", "nts", "nts (cryptographically authenticated, default) or insecure")
    allowInsecure := flag.Bool("allow-insecure", false, "explicitly opt into unauthenticated legacy NTP")
	maxStep := flag.Duration("max-step", 10*time.Minute, "refuse clock corrections larger than this")
	flag.Parse()
	if *mode != "sync" && *mode != "check" {
		fmt.Fprintln(os.Stderr, "-mode must be sync or check")
		os.Exit(2)
	}
	if *auth != "nts" && *auth != "insecure" {
        fmt.Fprintln(os.Stderr, "-auth must be nts or insecure")
        os.Exit(2)
    }
    if *auth == "insecure" && !*allowInsecure {
        fmt.Fprintln(os.Stderr, "refusing unauthenticated NTP: -auth insecure requires -allow-insecure")
        os.Exit(2)
    }
    if *peers == "" {
        if *auth == "nts" {
            *peers = "time.cloudflare.com,nts.netnod.se,nts.meta.com"
        } else {
            *peers = "time.cloudflare.com,time.windows.com,0.es.pool.ntp.org,1.es.pool.ntp.org"
        }
    }
    if *maxStep <= 0 {
		fmt.Fprintln(os.Stderr, "-max-step must be positive")
		os.Exit(2)
	}
	servers, err := validateServers(*peers)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	query := ntp.QueryNTS
    if *auth == "insecure" { query = ntp.Query }
    status, agreed, err := synchronize(ctx, servers, *mode, *maxStep, *auth == "nts", query, setClock)
	if *mode == "sync" {
		writeStatus(status)
	}
	output, _ := json.MarshalIndent(status, "", "  ")
	fmt.Println(string(output))
	if err != nil {
		fmt.Fprintln(os.Stderr, "time synchronization failed:", err)
		os.Exit(1)
	}
	fmt.Printf("Agreed NTP servers: %d; source validation complete (security: %s).\n", agreed, status.Security)
}
