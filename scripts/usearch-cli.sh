#!/usr/bin/env bash
# usearch CLI wrapper — guarantees the full backing stack is up and the host
# process env carries the adapter credentials before running the binary.
#
# The usearch binary reads process env only (not .env), and most adapters
# (X, Bluesky, youtube, searxng, ...) need the compose stack + env keys. This
# wrapper makes both available on every invocation. Alias `usearch` to it.
#
# ponytail: `compose up -d` (no --wait) is idempotent and sub-second when the
# stack is already running, so it is cheap to run on every call. --wait is
# deliberately omitted: it blocks ~2min whenever a non-essential service is
# slow/unhealthy (e.g. litellm), which the query does not need. First cold start
# may race the first query for a service still booting — just re-run. No
# daemon/LaunchAgent needed.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE=(docker compose -f "$REPO/deploy/docker-compose.yml")

# Load adapter credentials/config from .env into the process env.
if [[ -f "$REPO/.env" ]]; then
	set -a
	# shellcheck disable=SC1091
	source "$REPO/.env"
	set +a
fi

# Ensure the full stack is started (idempotent, no --wait — see note above).
# Degrade gracefully if Docker is unavailable so offline adapters still work.
if docker info >/dev/null 2>&1; then
	"${COMPOSE[@]}" up -d >/dev/null 2>&1 || \
		echo "usearch: warning: some stack services failed to start" >&2
else
	echo "usearch: warning: Docker not running; stack-backed adapters will fail" >&2
fi

exec "$REPO/usearch" "$@"
