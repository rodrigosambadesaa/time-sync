# TimeSync for Windows 11

A small, open-source, **no-third-party-dependencies** Windows clock synchronizer. Uses a native Win32 call (`SetSystemTime`) after a defensive NTPv4/SNTP time measurement; no browser scraping, cloud account, telemetry, or long-running background service.

> **Time.is is a comparison reference, not a public NTP service.** Time.is recommends using a nearby NTP time server for the computer clock (https://time.is/FAQ). TimeSync does **not** claim to connect to an official Time.is API or to have sub-millisecond accuracy. Compare the result visually at https://time.is/.

## Features

- Boot trigger (~30 seconds after startup), plus an indefinitely recurring Task Scheduler trigger **every 5 minutes**, even if no user logs in.
- Works as Windows **SYSTEM** because correcting the computer clock needs the system-time privilege.
- Parallel queries to four distinct NTP names, validated responses (NTP mode, leap/stratum, origin token, dispersion, round-trip delay) and **2+ server consensus**; rejects disagreements >150 ms.
- UTC clock correction with millisecond resolution. Less than 5 ms discrepancy is deliberately ignored to avoid incessant tiny clock steps; any correction >10 minutes is refused.
- Self-contained Windows x64 and ARM64 executables, zero Go runtime installation, offline uninstall.
- Minimal status in `%ProgramData%\TimeSync\status.json` (no personal data); no network access except NTP over UDP/123.

## Install

1. Download the release ZIP for your processor from **Releases**, and compare its SHA-256 with `SHA256SUMS.txt`.
2. **Extract the ZIP into a local folder you trust**. Do not run scripts directly from a compressed archive.
3. Open **Windows PowerShell as Administrator** in the extracted folder and run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1`. v0.1.1 fixes PowerShell parameter initialization and XML task-registration failures in v0.1.0.
4. Inspect Task Scheduler for `TimeSync-FiveMinutes`. This task executes the installed binary from `%ProgramFiles%\TimeSync` as SYSTEM. It runs immediately after installation as well.
5. Monitor `%ProgramData%\TimeSync\status.json` and compare https://time.is/ after the first successful sync.

Check without changing the clock: `& "$env:ProgramFiles\TimeSync\time-sync.exe" -mode check`.

Uninstall as Administrator: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\uninstall.ps1` (optional `-RemoveStatus`).

## Technical behavior / safety limitations

The **NTP protocol is not authenticated** here; MITM attacks and compromised servers cannot be completely excluded. Consensus and the 10-minute step cap reduce, but do not eliminate, risk. Do not use this for financial, industrial, security-critical or domain-joined timekeeping. Installation is intentionally blocked on Active Directory domain-joined PCs. A regular PC may already synchronize via the built-in Windows Time service; avoid deploying multiple competing time agents. NTP/UDP port 123 must be available; if blocked, the program leaves the clock untouched and reports a failure. It also leaves the clock untouched if sources disagree.

The source clock is only adjusted when the consensus offset exceeds 5 ms, and **each correction steps** the Windows clock rather than gradually slewing it. An occasional backward time step can affect software that relies on wall-clock time; Windows' monotonic clocks remain preferable for measuring elapsed time.

The default peers are `time.cloudflare.com`, `time.windows.com`, `0.es.pool.ntp.org` and `1.es.pool.ntp.org`. You can run a manual check with `-servers "a.example,b.example"`, but the scheduled task uses the built-in defaults. Time.is's own nominal web precision is typically about 20–100 ms, not a hard guarantee.

## Developer commands

```powershell
go test ./...
go vet ./...
$env:GOOS='windows'; $env:GOARCH='amd64'; go build -trimpath -o time-sync.exe ./cmd/time-sync
```

The `release.yml` workflow tests on Windows/Linux, validates PowerShell syntax, cross-compiles x64/ARM64, and publishes `v0.1.1` after a passing push to `main` (if absent). Later versions may be released by workflow dispatch with a version input.

## License

MIT; see [LICENSE](LICENSE).
