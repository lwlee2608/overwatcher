#!/bin/sh

# Extract DNS resolver from /etc/resolv.conf for nginx dynamic resolution
# Wrap IPv6 addresses in brackets as required by nginx
RAW=$(grep nameserver /etc/resolv.conf | head -1 | awk '{print $2}')
if echo "$RAW" | grep -q ':'; then
    export DNS_RESOLVER="[$RAW]"
else
    export DNS_RESOLVER="$RAW"
fi

# Run the default nginx entrypoint
exec /docker-entrypoint.sh "$@"
