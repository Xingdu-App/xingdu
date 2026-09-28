#!/bin/sh
set -eu
# Never print the service environment. Only DNS hostname:port is accepted;
# URLs, credentials, whitespace and nginx directives are rejected before rendering.
upstream=${XINGDU_API_UPSTREAM:-}
case "$upstream" in
  ""|*[!a-zA-Z0-9.:-]*) echo "Invalid XINGDU_API_UPSTREAM" >&2; exit 1 ;;
esac
if ! printf '%s' "$upstream" | awk '
 BEGIN { ok=0 }
 /^[a-zA-Z0-9.-]+:[0-9]+$/ {
   if (NF != 1) exit 1;
   split($0,parts,":"); if(parts[2]+0 < 1 || parts[2]+0 > 65535 || length(parts[1]) > 253) exit 1;
   count=split(parts[1],labels,".");
   for(i=1;i<=count;i++) if(length(labels[i])<1 || length(labels[i])>63 || labels[i] !~ /^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$/) exit 1;
   ok=1;
 }
 END { if(NR!=1 || !ok) exit 1 }
'; then
  echo 'XINGDU_API_UPSTREAM must be a private DNS hostname and port' >&2
  exit 1
fi
# Use the DNS servers provided by the actual runtime. Docker, Kubernetes and
# Zeabur do not necessarily use the same DNS resolver address.
XINGDU_DNS_RESOLVERS=$(awk '$1 == "nameserver" && $2 ~ /^[0-9a-fA-F:.]+$/ {
 if(index($2,":")) printf "[%s] ",$2; else printf "%s ",$2
}' /etc/resolv.conf)
if [ -z "$XINGDU_DNS_RESOLVERS" ]; then
  echo 'No usable runtime DNS resolver was found' >&2
  exit 1
fi
export XINGDU_API_UPSTREAM XINGDU_DNS_RESOLVERS
envsubst '${XINGDU_API_UPSTREAM} ${XINGDU_DNS_RESOLVERS}' < /opt/xingdu/nginx.conf.template > /etc/nginx/conf.d/default.conf
nginx -t
