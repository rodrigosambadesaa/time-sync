#!/usr/bin/env bash
set -euo pipefail
if (( EUID != 0 )); then echo 'Run with sudo/root.' >&2; exit 1; fi
if [[ $# -gt 1 ]] || { [[ $# -eq 1 ]] && [[ "$1" != '--remove-status' ]]; }; then echo 'Usage: uninstall-linux.sh [--remove-status]' >&2; exit 2; fi
systemctl disable --now time-sync.timer 2>/dev/null || true
systemctl stop time-sync.service 2>/dev/null || true
rm -f /etc/systemd/system/time-sync.service /etc/systemd/system/time-sync.timer
rm -f /usr/lib/systemd/system-sleep/time-sync
systemctl daemon-reload
rm -f /usr/local/libexec/time-sync/time-sync
rmdir /usr/local/libexec/time-sync 2>/dev/null || true
if [[ ${1:-} == '--remove-status' ]]; then rm -rf /var/lib/time-sync; fi
echo 'TimeSync removed. Other time services were not changed.'
