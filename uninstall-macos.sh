#!/usr/bin/env bash
set -euo pipefail
if (( EUID != 0 )); then echo 'Run with sudo/root.' >&2; exit 1; fi
if [[ "$(uname -s)" != 'Darwin' ]]; then echo 'This uninstaller is for macOS.' >&2; exit 1; fi
if [[ $# -gt 1 ]] || { [[ $# -eq 1 ]] && [[ "$1" != '--remove-status' ]]; }; then echo 'Usage: uninstall-macos.sh [--remove-status]' >&2; exit 2; fi
label='com.rodrigosambadesaa.time-sync'
if launchctl print "system/$label" >/dev/null 2>&1; then
  launchctl bootout "system/$label"
fi
rm -f "/Library/LaunchDaemons/$label.plist"
rm -f /usr/local/libexec/time-sync/time-sync
rmdir /usr/local/libexec/time-sync 2>/dev/null || true
if [[ ${1:-} == '--remove-status' ]]; then rm -rf /var/db/time-sync; fi
echo 'TimeSync removed. macOS automatic time settings were not modified.'
