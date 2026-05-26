#!/usr/bin/env bash
# Wrapper for pre-commit ESLint hook. Strips the leading "web/" from each
# file argument so paths resolve correctly when ESLint is invoked from
# inside the web/ workspace directory.
set -euo pipefail

args=()
for f in "$@"; do
  args+=("${f#web/}")
done

# Nothing to lint (all args were outside web/?) — exit cleanly.
if [ "${#args[@]}" -eq 0 ]; then
  exit 0
fi

exec pnpm -C web exec eslint --max-warnings=0 --no-warn-ignored "${args[@]}"
