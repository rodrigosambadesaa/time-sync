#!/usr/bin/env bash
set -euo pipefail

if (( EUID != 0 )); then echo 'Run with sudo/root.' >&2; exit 1; fi
if ! command -v systemctl >/dev/null 2>&1; then echo 'systemd is required.' >&2; exit 1; fi
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

# Avoid silently replacing a managed system time service. Disable any existing
# time synchronization manually and deliberately before installing this agent.
for service in systemd-timesyncd.service chronyd.service chrony.service ntp.service ntpd.service openntpd.service; do
  if systemctl is-active --quiet "$service"; then
    echo "Another time synchronization service is active: $service. Disable it first." >&2
    exit 1
  fi
done

install -d -m 0755 /usr/local/libexec/time-sync
install -m 0755 "$binary" /usr/local/libexec/time-sync/time-sync
install -m 0644 "$script_dir/packaging/linux/time-sync.service" /etc/systemd/system/time-sync.service
sed "s/@INTERVAL@/$minutes/g" "$script_dir/packaging/linux/time-sync.timer.in" > /etc/systemd/system/time-sync.timer
chmod 0644 /etc/systemd/system/time-sync.timer
systemctl daemon-reload
systemctl enable --now time-sync.timer
printf 'Installed TimeSync. systemd timer: every %s minutes; status: /var/lib/time-sync/status.json\n' "$minutes"
