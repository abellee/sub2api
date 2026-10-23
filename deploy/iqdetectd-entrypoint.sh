#!/bin/sh
set -e

# Named volumes are root-owned. Fix the iqdetect data dir, then drop privileges.
if [ "$(id -u)" = "0" ]; then
    mkdir -p /app/data/iqdetect
    chown -R sub2api:sub2api /app/data 2>/dev/null || true
    exec su-exec sub2api "$0" "$@"
fi

exec /app/iqdetectd "$@"
