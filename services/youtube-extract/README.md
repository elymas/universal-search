# youtube-extract

YouTube extraction sidecar for Universal Search. Uses yt-dlp as a subprocess
to search YouTube and return structured JSON matching the Go adapter's wire
contract.

## Purpose

Provides `GET /health` and `POST /search` endpoints consumed by the Go YouTube
adapter (`internal/adapters/youtube/`). Runs yt-dlp as a subprocess for GPL
process-isolation (Apache-2.0 boundary preserved).

## Development

```bash
# Install with pip (from this directory)
pip install -e ".[dev]"

# Run tests (no live network — all yt-dlp calls are mocked)
pytest

# Lint
ruff check .

# Run locally
python -m youtube_extract
```

## Environment Variables

See `.env.example` for all available variables.

Key variables:
- `YT_EXTRACT_PORT` — listen port (default 8084)
- `YT_COOKIES_PATH` — optional cookies file for IP-block mitigation
- `YT_SLEEP_REQUESTS` / `YT_SLEEP_INTERVAL` / `YT_MAX_SLEEP_INTERVAL` — rate-limiting flags

## yt-dlp Version Pin

`pyproject.toml` pins yt-dlp to an exact version (`yt-dlp==2026.03.17`).
This is intentional — yt-dlp's YouTube extractor breaks roughly quarterly.

### Upgrade Procedure

1. Bump the version in `pyproject.toml` on a dedicated branch
2. Run the sidecar pytest suite: `pytest`
3. Smoke-test against a stable video ID:
   ```bash
   yt-dlp --dump-json --flat-playlist "ytsearch1:rick astley never gonna give you up" | head -1
   ```
4. If tests + smoke pass, update the pin and merge

## Wire Contract

The `/search` and `/health` JSON schemas are frozen by the Go adapter structs.
See SPEC-ADP-005a section 6.4 for the authoritative field-by-field contract.

## License

Apache-2.0 (yt-dlp is invoked as a subprocess, not linked).
