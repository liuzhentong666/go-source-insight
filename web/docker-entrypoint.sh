#!/bin/sh
set -eu
WEB_PROXY_TIMEOUT_SECONDS="${WEB_PROXY_TIMEOUT_SECONDS:-330}"
case "$WEB_PROXY_TIMEOUT_SECONDS" in
    ''|*[!0-9]*) echo 'WEB_PROXY_TIMEOUT_SECONDS must be an integer between 30 and 3600.' >&2; exit 1 ;;
esac
if [ "${#WEB_PROXY_TIMEOUT_SECONDS}" -gt 4 ] || [ "$WEB_PROXY_TIMEOUT_SECONDS" -lt 30 ] || [ "$WEB_PROXY_TIMEOUT_SECONDS" -gt 3600 ]; then
    echo 'WEB_PROXY_TIMEOUT_SECONDS must be an integer between 30 and 3600.' >&2
    exit 1
fi
export WEB_PROXY_TIMEOUT_SECONDS
# Only substitute the explicit setting, retaining Nginx's own $uri/$host variables.
envsubst '${WEB_PROXY_TIMEOUT_SECONDS}' < /etc/nginx/nginx.conf.template > /tmp/nginx.conf
exec nginx -c /tmp/nginx.conf -g 'daemon off;'
