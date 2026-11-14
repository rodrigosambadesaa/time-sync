# TimeSync — Windows, Linux and macOS

TimeSync is a small, open-source **Go standard-library-only** clock synchronizer for Windows, Linux and macOS (x64 and ARM64). It consults several NTPv4/SNTP servers, validates replies and requires a close consensus before adjusting system time. It does not scrape websites, collect telemetry, or require a cloud account.

> **Time.is is a manual visual comparison, not an NTP source.** TimeSync does not use a Time.is API and makes no guarantee of sub-millisecond accuracy. Visit https://time.is/ to compare clocks.

## Security and behavior

- Parallel NTP/UDP 123 exchanges with unique server hostnames; validates NTP server mode, leap/stratum, original request timestamp, root dispersion and latency.
- Requires at least two distinct servers with offsets within 150 ms; discards outliers. Refuses to move the clock more than 10 minutes by default (`-max-step`).
- `-mode check` measures and reports offset **without changing the system clock or writing a status file**. `-mode sync` applies the correction and records status.
- Each correction **steps** the OS clock (it is not gradual slewing). The schedule is every minute by default and is configurable at install time.
- Prebuilt, self-contained x64 and ARM64 executables for all three operating systems. Verify the release ZIP against `SHA256SUMS.txt`.

**Important:** NTP is **not cryptographically authenticated** here. Two agreeing servers do not protect against an on-path attacker. Do not deploy this on domain-managed, security-critical or regulated systems; use a managed, authenticated time source instead. UDP/123 and DNS must be reachable. **Do not run TimeSync alongside Windows Time, systemd-timesyncd, chrony, ntpd or macOS automatic network time.** Installers never disable those existing services automatically. The Linux installer rejects a running common Linux time service; the macOS installer checks the automatic network-time setting. If sources disagree, no clock change is made.

## Download and check

Download the matching ZIP from [Releases](https://github.com/rodrigosambadesaa/time-sync/releases):

| Operating system | Architecture | Archive |
|---|---|---|
| Windows | x64 / ARM64 | `time-sync-windows-amd64.zip` / `time-sync-windows-arm64.zip` |
| Linux | x64 / ARM64 | `time-sync-linux-amd64.zip` / `time-sync-linux-arm64.zip` |
| macOS | Intel / Apple Silicon | `time-sync-darwin-amd64.zip` / `time-sync-darwin-arm64.zip` |

Extract the ZIP into a trusted local folder first. On Linux or macOS, check the downloaded archive's SHA-256 with `shasum -a 256 archive.zip` (or `sha256sum archive.zip` on Linux), then compare with `SHA256SUMS.txt`.

## Windows 11

Open PowerShell **as Administrator** in the extracted folder:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -Interval 1m
```

This installs `%ProgramFiles%\TimeSync\time-sync.exe` and registers `TimeSync-OnBoot` and `TimeSync-Periodic` Task Scheduler tasks running as SYSTEM. Check `%ProgramData%\TimeSync\status.json` after the first run; the periodic task also catches up after sleep. The installer blocks domain-joined PCs.

```powershell
& "$env:ProgramFiles\TimeSync\time-sync.exe" -mode check
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\uninstall.ps1
```

Pass `-RemoveStatus` to the uninstaller to delete saved status information.

## Linux (systemd)

Requires a Linux distribution using systemd, root privileges for installation and `CAP_SYS_TIME` to adjust the clock. **First disable any pre-existing NTP service yourself**; for example, inspect `timedatectl` and `systemctl status systemd-timesyncd chrony` and follow your distribution's documented process. The installer refuses to proceed if it finds an active common time-sync service.

From the extracted ZIP:

```bash
sudo bash ./install-linux.sh --interval 1m
systemctl status time-sync.timer
sudo journalctl -u time-sync.service --no-pager -n 30
/usr/local/libexec/time-sync/time-sync -mode check
```

This installs the binary in `/usr/local/libexec/time-sync/`, a root-owned oneshot `time-sync.service` and a `time-sync.timer` running shortly after boot and periodically. The state file is `/var/lib/time-sync/status.json`. Only `-mode sync` requires clock-setting privileges; `-mode check` does not.

```bash
sudo bash ./uninstall-linux.sh
# Optional removal of the status directory:
sudo bash ./uninstall-linux.sh --remove-status
```

The installer does **not** change other time services; consider restoring your distribution's default time synchronization after uninstalling.

## macOS (launchd)

Requires root privileges for installation. On macOS, first disable **Set time and date automatically** in System Settings. TimeSync creates a root-owned LaunchDaemon (running at load and at the configured interval) and uses the Unix `settimeofday` system call.

```bash
sudo bash ./install-macos.sh --interval 1m
sudo launchctl print system/com.rodrigosambadesaa.time-sync
/usr/local/libexec/time-sync/time-sync -mode check
cat /var/db/time-sync/status.json
```

The installed binary is `/usr/local/libexec/time-sync/time-sync`; the configuration is `/Library/LaunchDaemons/com.rodrigosambadesaa.time-sync.plist`. To uninstall:

```bash
sudo bash ./uninstall-macos.sh
# Optional removal of saved status:
sudo bash ./uninstall-macos.sh --remove-status
```

Re-enable Apple's automatic time service afterward if desired. The installer does not adjust macOS time-service preferences. macOS may deny clock-setting even to elevated processes under managed security policies; failures are surfaced in the result and status file.

## Running manually

```bash
./time-sync -mode check
sudo ./time-sync -mode sync
./time-sync -mode check -servers "time.cloudflare.com,time.google.com,0.pool.ntp.org"
```

Use the `.exe` suffix on Windows. Servers must be 2–8 distinct nonempty hostnames. A local check is non-privileged and never modifies the clock. Both Linux and macOS installations accept `--interval Nm`, with whole minutes `1m`–`1439m`.

## Verification and limitations

CI tests Go logic on Linux, Windows and macOS, exercises a synthetic local UDP NTP server, tests decision-making with a mocked clock setter (including refusal cases), validates installer formats, runs a Windows scheduled-task installation smoke test, and cross-compiles all six release binaries. Tests **do not actually move the macOS/Linux hosted runner's clock** or exercise every distribution's systemd/launchd installation; perform a supervised `sudo ... -mode sync` check on each production platform after disabling competing time agents. A successful cross-compilation alone is not proof of privileged clock updates.

Developer commands:

```bash
go test ./...
go vet ./...
go test -race ./... # Linux
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build ./cmd/time-sync
```

## License

MIT, see [LICENSE](LICENSE).
