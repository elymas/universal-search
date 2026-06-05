---
name: sidecar-port-topology
description: Python sidecar port assignments in the docker-compose stack and the YouTube 8082 collision that SPEC-ADP-005a fixes
metadata:
  type: project
---

Python sidecars in `deploy/docker-compose.yml` occupy: searxng 8080, researcher 8081, embedder 8082, tokenizer-ko 8083. Port 8084 is FREE in the compose stack (storm uses 8084 only in `charts/universal-search/values.yaml`, the Helm chart, which has no docker-compose block).

The implemented YouTube Go adapter (`internal/adapters/youtube/`, SPEC-ADP-005) hardcoded `defaultBaseURL = http://localhost:8082` (`youtube.go:20`), colliding with embedder. Commit 742564d gated YouTube registration behind `YOUTUBE_BASE_URL` (`cmd/usearch/query.go:488`) as a stop-gap, so YouTube silently no-ops when unset. The sidecar `services/youtube-extract/` was contractually documented in ADP-005 §6.4 but never built (deferred via ADP-005 Open Question §11.7).

SPEC-ADP-005a (drafted 2026-06-04) builds the sidecar on port 8084 and changes the Go default to :8084.

**Why:** future SPECs touching sidecars or YouTube need the port map and must know YouTube search is non-functional end-to-end until ADP-005a ships.

**How to apply:** when assigning a new sidecar port, 8085+ is next free; when reasoning about why YouTube returns nothing, it is the missing sidecar + registration gate, not an adapter bug. The wire contract is FROZEN by the Go structs in `search.go`/`parse.go`/`youtube.go` — sidecars conform to Go, not the reverse. See [[spec-stale-code-assumptions]] — always verify cited paths before implementing.
