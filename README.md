# TimeSync — authenticated network time for Windows, Linux and macOS

TimeSync adjusts **the operating system's software clock**, not external watches, hardware clocks, or every embedded device. It supports Windows, Linux and macOS (amd64 and arm64).

**Starting with v0.3.0, [RFC 8915 Network Time Security (NTS)](https://www.rfc-editor.org/rfc/rfc8915.html) is the default.** A TLS 1.3 NTS key exchange authenticates the server, and authenticated encryption (AEAD) protects the NTP response. A TLS handshake **alone** is insufficient; TimeSync accepts samples only after the authenticated NTP response passes verification.

## Security modes and limitations

- **Default `-auth nts`:** only NTS-authenticated time samples participate in consensus. Requires two or more distinct authenticated sources agreeing within 150 ms; no automatic or silent downgrade to unauthenticated NTP if certificate validation, key exchange, AEAD or replay protection fails.
- **Legacy `-auth insecure -allow-insecure`:** explicitly opts in to unauthenticated NTP for diagnostics/compatibility. Its sanity checks and multi-server consensus are **not cryptographic security**. Not recommended on untrusted networks.
- **`-mode check`:** reads the offset and reports results without modifying the host clock. **`-mode sync`:** applies a correction only after validation and consensus, and writes JSON status.
- Clock updates **step**, rather than slew, system time. A correction above `-max-step` (default 10 minutes) is refused; choose a smaller threshold where appropriate. Stepping can break TLS, event ordering and time-sensitive applications.

The default NTS sources, each from a different operator, are `time.cloudflare.com` (Cloudflare), `nts.netnod.se` (Netnod) and `nts.meta.com` (Meta). You can specify 2–8 unique NTS key-exchange hosts via `-servers`. The network must allow **TCP port 4460** for NTS key exchange, **UDP port 123** for authenticated NTP and DNS; initial local time should be accurate enough for TLS certificate verification. If fewer than two NTS sources verify and agree, **the system clock is not changed**.

NTS mitigates in-transit spoofing but **does not certify TimeSync for safety-critical clocks**. Compromised endpoints, dishonest time providers, software defects, supply-chain attacks, wrong hardware RTC, leap smearing, GNSS/PTP, monitoring and TLS bootstrapping remain relevant. For regulated, medical, financial, industrial, domain-managed or other sensitive time infrastructure, use a properly engineered, monitored and audited time architecture (for example, approved chrony/NTPsec with NTS and independent time sources). Neither NTS nor this release alone guarantees precision, integrity or regulatory compliance.

NTS uses the maintained third-party [beevik/nts](https://github.com/beevik/nts) Go client (version pinned in `go.mod` and checksummed in `go.sum`), rather than inventing cryptography. Time.is is only a **manual visual comparison reference** and is not queried as an NTP source.

## Downloads and verification

Download the correct `time-sync-{windows,linux,darwin}-{amd64,arm64}.zip` from [Releases](https://github.com/rodrigosambadesaa/time-sync/releases). Compare the SHA-256 digest of the archive with `SHA256SUMS.txt`, then extract into a trusted local directory. Binaries are not code-signed; review the source and GitHub Actions provenance before privileged installation.

## Installation

**Do not run concurrent time-setting agents.** Inspect existing time synchronization before making a deliberate migration. Installers never silently disable other time services. Only install on machines you administer.

### Windows 11 (Task Scheduler)

Open an elevated PowerShell in the extracted archive:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\install.ps1 -Interval 1m
& "$env:ProgramFiles\TimeSync\time-sync.exe" -mode check
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\uninstall.ps1
```

Runs `TimeSync-Periodic` and `TimeSync-OnBoot` as SYSTEM. Domain-joined installations are blocked. Status is written to `%ProgramData%\TimeSync\status.json` in synchronization mode.

### Linux (systemd)

Inspect `timedatectl`, `systemctl status systemd-timesyncd chrony chronyd` and disable conflicting agents **manually**. The installer rejects common active time services.

```bash
sudo bash ./install-linux.sh --interval 1m
systemctl status time-sync.timer
/usr/local/libexec/time-sync/time-sync -mode check
sudo journalctl -u time-sync.service --no-pager -n 30
sudo bash ./uninstall-linux.sh
```

Status: `/var/lib/time-sync/status.json`. Adjusting system time requires root or `CAP_SYS_TIME`.

### macOS (launchd)

Disable automatic network time in System Settings before installing; the installer checks this setting. Root privileges are required for OS clock updates.

```bash
sudo bash ./install-macos.sh --interval 1m
sudo launchctl print system/com.rodrigosambadesaa.time-sync
/usr/local/libexec/time-sync/time-sync -mode check
sudo bash ./uninstall-macos.sh
```

Status: `/var/db/time-sync/status.json`. Re-enable Apple's time service when uninstalling if desired.

## Manual use

```bash
./time-sync -mode check
sudo ./time-sync -mode sync -max-step 2s
./time-sync -mode check -servers "time.cloudflare.com,nts.netnod.se,nts.meta.com"
./time-sync -mode check -auth insecure -allow-insecure  # diagnostics only!
```

Append `.exe` on Windows. Scheduled installations use the secure default, without insecure opt-in.

## Testing and supported assurance

```bash
go test ./...
go vet ./...
go test -race ./...  # Linux
```

GitHub Actions runs these checks on Windows, Linux and macOS, verifies secure-mode rejection of unauthenticated/mixed-source samples and invalid NTS TLS certificates, smoke-tests CLI defaults, validates native installer formats, and builds all six archives.

The automated tests **do not change the hosted runner's real clock**, conduct formal NTS cryptography audits, validate every systemd/launchd configuration or prove regulatory/safety-critical fitness. Perform privileged synchronization tests on managed machines, with rollback and monitoring, before deployment.

## License

MIT; see [LICENSE](LICENSE).
