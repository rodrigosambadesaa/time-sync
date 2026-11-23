# TimeSync — authenticated network time for Windows, Linux, macOS and rooted Android

TimeSync adjusts **the operating system's software clock**, not external watches, hardware clocks, or every embedded device. It supports Windows, Linux and macOS (amd64 and arm64), plus rooted Android (arm64 command-line build). A jailbroken iPhone/iPad can be synchronized remotely over authenticated SSH from a Windows/Linux/macOS controller; ordinary iOS/iPadOS applications cannot set the system time.

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

Download the correct `time-sync-{windows,linux,darwin}-{amd64,arm64}.zip or time-sync-android-arm64.zip` from [Releases](https://github.com/rodrigosambadesaa/time-sync/releases). Compare the SHA-256 digest of the archive with `SHA256SUMS.txt`, then extract into a trusted local directory. Binaries are not code-signed; review the source and GitHub Actions provenance before privileged installation.

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

### Rooted Android (command-line, not an APK)

Use the `time-sync-android-arm64.zip` archive for most modern rooted phones. The Android executable runs locally on-device; it is **not** an APK, Android service or Play Store app. Obtain a trusted root shell (for example via an already-configured Magisk `su`), copy the executable into a private, executable directory, and run it under root:

```sh
su -c '/data/adb/time-sync/time-sync -mode check'
su -c '/data/adb/time-sync/time-sync -mode sync -max-step 2s'
```

Install the binary in the root-owned `/data/adb/time-sync` directory using restrictive permissions (for example `0700`), not in a world-writable location. Check that the device permits execution from that directory. The clock is set via the Linux `settimeofday` syscall, subject to Android SELinux and `CAP_SYS_TIME`: **root does not guarantee success**. Status is stored at `/data/adb/time-sync/status.json` if that root-only path is available. Disable Android's automatic network time before testing so Android does not immediately overwrite the correction. No installer or always-on background process is provided; start with manual verification on your own device. Do not disable SELinux to make clock updates work.

### iPhone / iPad (jailbreak only, via SSH)

**Stock iOS/iPadOS:** third-party App Store apps have no public API for setting the system clock. No rooting or jailbreak bypass is offered. Change date/time in Settings manually or use the OS's automatic time setting.

**Jailbroken devices:** TimeSync v0.4.0 provides an optional **desktop-controller** path, not a native IPA. You must *already* have a supported jailbreak, an SSH server with a verified host key, key-based **root** access, and a privileged BSD-compatible `date` command permitted to update the clock. On your desktop, check the SSH host key through an independent trusted channel, then run:

```sh
./time-sync -mode sync -target-ssh root@iphone.local -max-step 2m
```

Windows users run `time-sync.exe` and need an OpenSSH client. The controller authenticates NTS samples from multiple providers, estimates the corrected UTC instant, queries the device clock and refuses any device correction above `-max-step` before running a fixed `date -u` command over SSH. SSH host-key verification is strict and password prompts are disabled. A jailbroken device may still reject setting time because of sandboxing, entitlements, codesigning or jailbreak-specific restrictions; **not validated on physical iOS devices**. This method has **second-level precision plus SSH latency**, not the accuracy of native NTS synchronization, and it should not be used for precision or security-critical timekeeping. Do not expose the phone's SSH service to the public Internet, and avoid competing iOS automatic time settings.

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

GitHub Actions runs these checks on Windows, Linux and macOS, verifies secure-mode rejection of unauthenticated/mixed-source samples and invalid NTS TLS certificates, smoke-tests CLI defaults, validates native installer formats, and builds all seven archives (six desktop plus one Android). Remote iOS tests use a fake SSH command runner, never a real device.

The automated tests **do not change the hosted runner's or a phone's real clock**, conduct formal NTS cryptography audits, validate every systemd/launchd configuration or prove regulatory/safety-critical fitness. Perform privileged synchronization tests on managed machines, with rollback and monitoring, before deployment.

## License

MIT; see [LICENSE](LICENSE).

## Wake and hibernation recovery (v0.4.1)

The installer now registers additional wake-aware scheduling. Network Time
Security (NTS) remains mandatory by default. When Wi-Fi/DNS/VPN need to reconnect,
wake-triggered synchronization retries failed secure attempts for **up to 45
seconds**, without allowing insecure fallback or bypassing two-source consensus.

- **Windows 10/11:** `TimeSync-OnResume` listens for System events from
  Power-Troubleshooter (ID 1) and Kernel-Power (ID 107). It starts a secure
  synchronization upon wake instead of waiting for `TimeSync-Periodic`.
  Update by extracting the new release and running an elevated
  `powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\\install.ps1 -Interval 1m`.
  Verify with `Get-ScheduledTask -TaskName 'TimeSync-*'` and inspect
  `Get-Content "$env:ProgramData\\TimeSync\\status.json"`.
- **Linux/systemd:** A root-owned post-resume system-sleep hook queues
  `time-sync.service` via `systemctl --no-block`. It intentionally performs
  **no networking inside the systemd-sleep hook**. Reinstall using
  `sudo bash ./install-linux.sh --interval 1m` and inspect
  `sudo journalctl -u time-sync.service --no-pager -n 40`.
- **macOS:** A wake-aware `StartCalendarInterval` replaces the previous
  `StartInterval`, which can miss triggers during sleep. Reinstall with
  `sudo bash ./install-macos.sh --interval 1m`. For intervals not dividing
  one hour, the calendar is aligned to midnight and the final daily interval
  can be shorter.
- **Android (rooted) and iOS (jailbroken):** The manual Android CLI and
  remote SSH iOS controller remain unchanged; neither has a background
  wake-resume integration.

For diagnosis, `time-sync -mode check` reports authenticated NTS offsets
without setting the clock; `time-sync -mode sync -retry-window 45s` retries
temporary failures while staying within the existing `-max-step` bound.
The default remains a single attempt when `-retry-window` is omitted.
**Neither six-millisecond absolute accuracy nor zero wake latency is
guaranteed:** a valid NTS sample needs network access and corroboration.

CI checks the Windows event subscription, mocks the Linux post-resume hook,
validates launchd calendar rendering on macOS, and runs Go tests on all three.
Hosted CI cannot actually hibernate hardware or validate its privileged clock
precision; perform a controlled resume test on a managed machine.
