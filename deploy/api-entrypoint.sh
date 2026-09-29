#!/bin/sh
set -eu

if [ "${1:-}" = "server" ]; then
  case "${XINGDU_AUTO_MIGRATE:-true}" in
    true)
      if [ -z "${XINGDU_MIGRATION_DATABASE_URL:-}" ]; then
        echo "API startup requires XINGDU_MIGRATION_DATABASE_URL for automatic migrations" >&2
        exit 1
      fi
      if [ -z "${DATABASE_URL:-}" ]; then
        echo "API startup requires a separate runtime DATABASE_URL" >&2
        exit 1
      fi
      # The privileged connection is only passed to the short-lived child.
      echo "Applying database migrations before API startup" >&2
      DATABASE_URL="$XINGDU_MIGRATION_DATABASE_URL" migrate
      ;;
    false) ;;
    *) echo "XINGDU_AUTO_MIGRATE must be true or false" >&2; exit 1 ;;
  esac
  # Do not pass migration-only credentials into the long-running API process.
  unset XINGDU_MIGRATION_DATABASE_URL XINGDU_APP_DATABASE_PASSWORD XINGDU_WORKER_DATABASE_PASSWORD
fi
exec "$@"
