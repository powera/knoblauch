#!/usr/bin/env bash
set -e

cd "$(dirname "$0")"

pkill -f bin/knoblauch 2>/dev/null && echo "Stopped existing knoblauch process" || echo "No running knoblauch process found"

exec ./launch.sh "$@"
