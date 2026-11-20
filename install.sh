#!/usr/bin/env bash
# Register niagara-git in a station repository and merge the starter templates.
# Usage, from the station repo or with its path:
#   ./install.sh
#   ./install.sh /path/to/station-repo
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [[ -n "${PYTHON:-}" ]]; then
  PY="$PYTHON"
elif command -v python3 >/dev/null 2>&1; then
  PY="python3"
elif command -v python >/dev/null 2>&1; then
  PY="python"
else
  echo "niagara-git: Python 3.8+ is required on PATH (or set PYTHON)." >&2
  exit 1
fi

exec "$PY" "$ROOT/niagara_git/__main__.py" install "$@"
