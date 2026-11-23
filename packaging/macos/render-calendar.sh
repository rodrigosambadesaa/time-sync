#!/usr/bin/env bash
# Build a launchd calendar that catches up on sleep/hibernate wake.
set -euo pipefail
if (( $# != 3 )); then
  echo 'Usage: render-calendar.sh TEMPLATE INTERVAL_MINUTES OUTPUT' >&2
  exit 2
fi
template="$1"; minutes="$2"; output="$3"
if [[ ! "$minutes" =~ ^[0-9]+$ ]] || (( minutes < 1 || minutes > 1439 )); then
  echo 'Interval must be 1-1439 minutes' >&2
  exit 2
fi
entries="$(mktemp)"
trap 'rm -f "$entries"' EXIT
if (( minutes <= 60 && 60 % minutes == 0 )); then
  for (( minute=0; minute<60; minute+=minutes )); do
    printf '    <dict><key>Minute</key><integer>%d</integer></dict>\n' "$minute" >> "$entries"
  done
else
  # Wall-clock alignment: final interval across midnight may be shorter.
  for (( at=0; at<1440; at+=minutes )); do
    printf '    <dict><key>Hour</key><integer>%d</integer><key>Minute</key><integer>%d</integer></dict>\n' "$((at / 60))" "$((at % 60))" >> "$entries"
  done
fi
awk -v entries="$entries" '
  /@CALENDAR_ENTRIES@/ { while ((getline line < entries) > 0) print line; close(entries); next }
  { print }
' "$template" > "$output"
