#!/usr/bin/env bash
# Locate or download the niagara-git binary and register it in a station repo.
# Usage, from the station repo or with its path:
#   ./install.sh
#   ./install.sh /path/to/station-repo
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

download_niagara_git() {
  local os arch asset repo version url dest tmp origin cache
  case "$(uname -s)" in
    Linux) os="linux" ;;
    Darwin) os="darwin" ;;
    MINGW*|MSYS*|CYGWIN*) os="windows" ;;
    *)
      echo "niagara-git: unsupported operating system $(uname -s). Download a release binary and set NIAGARA_GIT." >&2
      exit 1
      ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *)
      echo "niagara-git: unsupported architecture $(uname -m)." >&2
      exit 1
      ;;
  esac
  if [[ "$os" == "windows" && "$arch" != "amd64" ]]; then
    echo "niagara-git: the Windows build is amd64 only." >&2
    exit 1
  fi
  asset="niagara-git-${os}-${arch}"
  if [[ "$os" == "windows" ]]; then
    asset="${asset}.exe"
  fi

  repo="${NIAGARA_GIT_REPO:-}"
  if [[ -z "$repo" ]]; then
    origin="$(git -C "$ROOT" remote get-url origin 2>/dev/null || true)"
    origin="${origin%.git}"
    if [[ "$origin" =~ github.com[:/]([^/]+/[^/]+)$ ]]; then
      repo="${BASH_REMATCH[1]}"
    fi
  fi
  repo="${repo:-bew1769/niagara-git}"
  version="${NIAGARA_GIT_VERSION:-latest}"
  if [[ "$version" == "latest" ]]; then
    url="https://github.com/${repo}/releases/latest/download/${asset}"
  else
    url="https://github.com/${repo}/releases/download/${version}/${asset}"
  fi

  cache="${XDG_CACHE_HOME:-$HOME/.cache}/niagara-git"
  mkdir -p "$cache"
  dest="$cache/$asset"
  tmp="$(mktemp)"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$tmp" "$url"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$tmp" "$url"
  else
    rm -f "$tmp"
    echo "niagara-git: curl or wget is required to download the binary, or place niagara-git next to install.sh." >&2
    exit 1
  fi
  chmod +x "$tmp"
  mv "$tmp" "$dest"
  printf '%s\n' "$dest"
}

if [[ -n "${NIAGARA_GIT:-}" ]]; then
  BIN="$NIAGARA_GIT"
elif [[ -x "$ROOT/niagara-git" ]]; then
  BIN="$ROOT/niagara-git"
elif [[ -x "$ROOT/niagara-git.exe" ]]; then
  BIN="$ROOT/niagara-git.exe"
elif command -v niagara-git >/dev/null 2>&1; then
  BIN="$(command -v niagara-git)"
else
  BIN="$(download_niagara_git)"
fi

exec "$BIN" install "$@"
