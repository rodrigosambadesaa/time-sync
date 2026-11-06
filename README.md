# TimeSync for Windows 11

A small, open-source, **no-third-party-dependencies** Windows clock synchronizer. Uses a native Win32 call (`SetSystemTime`) after a defensive NTPv4/SNTP time measurement; no browser scraping, cloud account, telemetry, or long-running background service.

> **Time.is is a comparison reference, not a public NTP service.** Time.is recommends using a nearby NTP time server for the computer clock (https://time.is/FAQ). TimeSync does **not** claim to connect to an official Time.is API or to have sub-millisecond accuracy. Compare the result visually at https://time.is/.

## Features

- Two native Task Scheduler tasks: `TimeSync-OnBoot` (~30 seconds after startup) and `TimeSync-FiveMinutes` (repeats **every 5 minutes by default** indefinitely), even when no user logs in. The interval is configurable during installation.
- Works as Windows **SYSTEM** because correcting the computer clock needs the system-time privilege.
- Parallel queries to four distinct NTP names, validated responses (NTP mode, leap/stratum, origin token, dispersion, round-trip delay) and **2+ server consensus**; rejects disagreements >150 ms.
- UTC clock correction with millisecond resolution. Less than 5 ms discrepancy is deliberately ignored to avoid incessant tiny clock steps; any correction >10 minutes is refused.
- Self-contained Windows x64 and ARM64 executables, zero Go runtime installation, offline uninstall.
- Minimal status in `%ProgramData%\TimeSync\status.json` (no personal data); no network access except NTP over UDP/123.

## Install

1. Download the release ZIP for your processor from **Releases**, and compare its SHA-256 with `SHA256SUMS.txt`.
2. **Extract the ZIP into a local folder you trust**. Do not run scripts directly from a compressed archive.
3. Open **Windows PowerShell as Administrator** in the extracted folder and run `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1`. v0.1.2 additionally replaces the invalid XML `LogonType` with native `schtasks` registration and tests installation on a Windows runner.
4. Inspect Task Scheduler for `TimeSync-FiveMinutes` and `TimeSync-OnBoot`. Both tasks execute the installed binary from `%ProgramFiles%\TimeSync` as SYSTEM. The installer also starts the first sync immediately.
5. Monitor `%ProgramData%\TimeSync\status.json` and compare https://time.is/ after the first successful sync.

The default interval is five minutes. To choose a custom interval, pass `-Interval` when installing; use a number followed by `s`, `m`, `h` or `d`, or `HH:MM:SS`:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -Interval 15m
```

Intervals from 1 minute through 23 hours 59 minutes are accepted; use a whole number of minutes because the installer uses Task Scheduler's native minute trigger. Re-running the installer updates the existing scheduled task.

Check without changing the clock: `& "$env:ProgramFiles\TimeSync\time-sync.exe" -mode check`.

Uninstall as Administrator: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\uninstall.ps1` (optional `-RemoveStatus`).

## Technical behavior / safety limitations

The **NTP protocol is not authenticated** here; MITM attacks and compromised servers cannot be completely excluded. Consensus and the 10-minute step cap reduce, but do not eliminate, risk. Do not use this for financial, industrial, security-critical or domain-joined timekeeping. Installation is intentionally blocked on Active Directory domain-joined PCs. A regular PC may already synchronize via the built-in Windows Time service; avoid deploying multiple competing time agents. NTP/UDP port 123 must be available; if blocked, the program leaves the clock untouched and reports a failure. It also leaves the clock untouched if sources disagree.

Every scheduled run re-measures the offset. The source clock is only adjusted when the consensus offset exceeds 10 ms by default (`-min-drift 10ms`), and **each correction steps** the Windows clock rather than gradually slewing it. You can raise or lower that threshold when invoking the binary, for example `-min-drift 50ms`. An occasional backward time step can affect software that relies on wall-clock time; Windows' monotonic clocks remain preferable for measuring elapsed time.

The default peers are `time.cloudflare.com`, `time.windows.com`, `0.es.pool.ntp.org` and `1.es.pool.ntp.org`. You can run a manual check with `-servers "a.example,b.example"`, but the scheduled task uses the built-in defaults. Time.is's own nominal web precision is typically about 20–100 ms, not a hard guarantee.

## Developer commands

```powershell
go test ./...
go vet ./...
$env:GOOS='windows'; $env:GOARCH='amd64'; go build -trimpath -o time-sync.exe ./cmd/time-sync
```

The `release.yml` workflow tests on Windows/Linux, validates PowerShell syntax, cross-compiles x64/ARM64, runs an actual Windows scheduled-task install/uninstall test, and publishes releases after a passing push to `main` (if absent). Later versions may be released by workflow dispatch with a version input.

## License

MIT; see [LICENSE](LICENSE).

## Upgrading from v0.1.0 or v0.1.1

Extract the new ZIP into a *new* directory and rerun `install.ps1` as Administrator. The installer replaces the binary and registers both tasks. Uninstall removes both tasks; no reboot is required. SHA256 checksums change with each release.
