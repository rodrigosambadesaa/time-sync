#!/usr/bin/env bash
set -euo pipefail
if (( EUID != 0 )); then echo 'Run with sudo/root.' >&2; exit 1; fi
if [[ "$(uname -s)" != 'Darwin' ]]; then echo 'This installer is for macOS.' >&2; exit 1; fi
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
binary="$script_dir/time-sync"
interval='1m'
while (( $# )); do
  case "$1" in
    --binary|--interval)
      if (( $# < 2 )); then echo "Missing value for $1" >&2; exit 2; fi
      case "$1" in --binary) binary="$2" ;; --interval) interval="$2" ;; esac
      shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 2 ;;
  esac
done
if [[ ! -f "$binary" ]]; then echo "Missing binary: $binary" >&2; exit 1; fi
if [[ ! "$interval" =~ ^([0-9]+)m$ ]]; then echo 'Interval must be 1m–1439m.' >&2; exit 2; fi
raw_minutes="${BASH_REMATCH[1]}"
if (( ${#raw_minutes} > 4 )); then echo 'Interval must be 1m–1439m.' >&2; exit 2; fi
minutes=$((10#$raw_minutes))
if (( minutes < 1 || minutes > 1439 )); then echo 'Interval must be 1m–1439m.' >&2; exit 2; fi

# macOS normally runs timed; explicitly disable automatic network time in
# System Settings before running TimeSync. Never alter that setting silently.
if systemsetup -getusingnetworktime 2>/dev/null | grep -qi ': on'; then
  echo 'macOS automatic network time is enabled. Disable it in System Settings before installing.' >&2
  exit 1
fi

label='com.rodrigosambadesaa.time-sync'
plist="/Library/LaunchDaemons/$label.plist"
install -d -m 0755 /usr/local/libexec/time-sync
install -m 0755 "$binary" /usr/local/libexec/time-sync/time-sync
bash "$script_dir/packaging/macos/render-calendar.sh" "$script_dir/packaging/macos/$label.plist.in" "$minutes" "$plist"
chown root:wheel "$plist" /usr/local/libexec/time-sync/time-sync
chmod 0644 "$plist"
plutil -lint "$plist"
if launchctl print "system/$label" >/dev/null 2>&1; then
  launchctl bootout "system/$label"
fi
launchctl bootstrap system "$plist"
printf 'Installed TimeSync LaunchDaemon (every %s minutes); status: /var/db/time-sync/status.json\n' "$minutes"
