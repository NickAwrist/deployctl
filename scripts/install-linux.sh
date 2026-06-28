#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Install deployctl and deployctld on Linux.

Usage:
  scripts/install-linux.sh
  curl -fsSL 'https://github.com/NickAwrist/deployctl/blob/main/scripts/install-linux.sh?raw=1' | bash

Environment:
  PREFIX=/usr/local/bin       Binary install directory. Defaults to /usr/local/bin as root, ~/.local/bin otherwise.
  SERVICE_SCOPE=system        system or user. Defaults to system as root, user otherwise.
  DEPLOYCTL_USER=<user>       User for the systemd system service. Defaults to SUDO_USER, then current user.
  DEPLOYCTL_GO_IMAGE=<image>  Go Docker image used when local go is unavailable. Defaults to golang:1.25.
  DEPLOYCTL_REPO_URL=<url>    Repository to clone when the script is downloaded. Defaults to the GitHub repo.
  DEPLOYCTL_REF=<ref>         Branch, tag, or commit to install when downloaded. Defaults to main.
  SYNC_EXISTING_CLIENTS=0     Do not update other writable deployctl client copies found on common PATHs.
  SKIP_SERVICE=1              Install binaries only; do not create/start a systemd service.

Update:
  git pull
  scripts/install-linux.sh    # or sudo scripts/install-linux.sh for a system service

Downloaded update:
  curl -fsSL 'https://github.com/NickAwrist/deployctl/blob/main/scripts/install-linux.sh?raw=1' | bash
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "error: install-linux.sh only supports Linux" >&2
  exit 1
fi

if ! command -v systemctl >/dev/null 2>&1 && [[ "${SKIP_SERVICE:-0}" != "1" ]]; then
  echo "error: systemctl is required unless SKIP_SERVICE=1 is set" >&2
  exit 1
fi

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT
build_dir="$tmp_dir/build"
mkdir -p "$build_dir"

DEFAULT_DEPLOYCTL_REPO_URL="https://github.com/NickAwrist/deployctl.git"
DEPLOYCTL_GO_IMAGE="${DEPLOYCTL_GO_IMAGE:-golang:1.25}"
DEPLOYCTL_REPO_URL="${DEPLOYCTL_REPO_URL:-$DEFAULT_DEPLOYCTL_REPO_URL}"
DEPLOYCTL_REF="${DEPLOYCTL_REF:-main}"

script_path="${BASH_SOURCE[0]:-}"
script_dir=""
if [[ -n "$script_path" ]]; then
  script_dir="$(cd "$(dirname "$script_path")" >/dev/null 2>&1 && pwd || true)"
fi
repo_root=""
if [[ -n "$script_dir" && -f "$script_dir/../go.mod" && -f "$script_dir/../main.go" ]]; then
  repo_root="$(cd "$script_dir/.." && pwd)"
else
  if ! command -v git >/dev/null 2>&1; then
    echo "error: git is required when installing from a downloaded script" >&2
    exit 1
  fi

  repo_root="$tmp_dir/source"
  echo "Downloading deployctl source from $DEPLOYCTL_REPO_URL ($DEPLOYCTL_REF)"
  git clone --quiet --depth 1 "$DEPLOYCTL_REPO_URL" "$repo_root"
  (
    cd "$repo_root"
    git fetch --quiet --depth 1 origin "$DEPLOYCTL_REF" >/dev/null 2>&1 || true
    git checkout --quiet "$DEPLOYCTL_REF"
  )
fi

is_root=0
if [[ "$(id -u)" == "0" ]]; then
  is_root=1
fi

if [[ -z "${SERVICE_SCOPE:-}" ]]; then
  if [[ "$is_root" == "1" ]]; then
    SERVICE_SCOPE="system"
  else
    SERVICE_SCOPE="user"
  fi
fi

if [[ "$SERVICE_SCOPE" != "system" && "$SERVICE_SCOPE" != "user" ]]; then
  echo "error: SERVICE_SCOPE must be system or user" >&2
  exit 1
fi

if [[ -z "${PREFIX:-}" ]]; then
  if [[ "$SERVICE_SCOPE" == "system" ]]; then
    PREFIX="/usr/local/bin"
  else
    PREFIX="$HOME/.local/bin"
  fi
fi

if [[ "$SERVICE_SCOPE" == "system" && "$is_root" != "1" ]]; then
  echo "error: SERVICE_SCOPE=system requires root. Re-run with sudo, or use SERVICE_SCOPE=user." >&2
  exit 1
fi

install_user="${DEPLOYCTL_USER:-${SUDO_USER:-$(id -un)}}"
if [[ "$SERVICE_SCOPE" == "system" ]]; then
  if ! id "$install_user" >/dev/null 2>&1; then
    echo "error: DEPLOYCTL_USER '$install_user' does not exist" >&2
    exit 1
  fi
  install_home="$(getent passwd "$install_user" | cut -d: -f6)"
else
  install_user="$(id -un)"
  install_home="$HOME"
fi

echo "Building deployctl from $repo_root"
if command -v go >/dev/null 2>&1; then
  (
    cd "$repo_root"
    go build -trimpath -ldflags "-s -w" -o "$build_dir/deployctl" .
    go build -trimpath -ldflags "-s -w" -o "$build_dir/deployctld" ./cmd/deployctld
  )
elif command -v docker >/dev/null 2>&1; then
  echo "Local go was not found; building with Docker image $DEPLOYCTL_GO_IMAGE"
  docker run --rm \
    -v "$repo_root:/src" \
    -v "$build_dir:/out" \
    -w /src \
    "$DEPLOYCTL_GO_IMAGE" \
    sh -c 'git config --global --add safe.directory /src && go build -trimpath -ldflags "-s -w" -o /out/deployctl . && go build -trimpath -ldflags "-s -w" -o /out/deployctld ./cmd/deployctld'
else
  echo "error: go or docker is required to build deployctl" >&2
  exit 1
fi

echo "Installing binaries to $PREFIX"
mkdir -p "$PREFIX"
install -m 0755 "$build_dir/deployctl" "$PREFIX/deployctl"
install -m 0755 "$build_dir/deployctld" "$PREFIX/deployctld"

sync_existing_clients() {
  if [[ "${SYNC_EXISTING_CLIENTS:-1}" == "0" ]]; then
    return
  fi

  local installed_client="$PREFIX/deployctl"
  local paths=()
  local resolved

  resolved="$(command -v deployctl 2>/dev/null || true)"
  if [[ -n "$resolved" ]]; then
    paths+=("$resolved")
  fi
  paths+=(
    "/usr/local/bin/deployctl"
    "$HOME/.local/bin/deployctl"
    "$install_home/.local/bin/deployctl"
  )

  local seen=":"
  local path
  for path in "${paths[@]}"; do
    if [[ -z "$path" || "$seen" == *":$path:"* || "$path" == "$installed_client" ]]; then
      continue
    fi
    seen="$seen$path:"

    if [[ ! -e "$path" ]]; then
      continue
    fi

    if [[ -w "$path" ]]; then
      if install -m 0755 "$build_dir/deployctl" "$path"; then
        echo "Updated existing deployctl client at $path"
      else
        echo "Notice: existing deployctl client at $path could not be updated." >&2
        echo "        Re-run with PREFIX=$(dirname "$path") using an account that can write there, or remove the stale copy from PATH." >&2
      fi
    else
      echo "Notice: existing deployctl client at $path was not updated because it is not writable." >&2
      echo "        Re-run with PREFIX=$(dirname "$path") using an account that can write there, or remove the stale copy from PATH." >&2
    fi
  done
}

sync_existing_clients

if [[ "${SKIP_SERVICE:-0}" == "1" ]]; then
  echo "Installed binaries only. Start the daemon manually with: $PREFIX/deployctld"
  exit 0
fi

if [[ "$SERVICE_SCOPE" == "system" ]]; then
  service_path="/etc/systemd/system/deployctld.service"
  echo "Writing systemd system service to $service_path"
  cat >"$service_path" <<EOF
[Unit]
Description=deployctl daemon
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
User=$install_user
Environment=HOME=$install_home
WorkingDirectory=$install_home
ExecStart=$PREFIX/deployctld
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable --now deployctld.service
  systemctl restart deployctld.service
  echo "Service status:"
  systemctl --no-pager --full status deployctld.service || true
else
  service_dir="$HOME/.config/systemd/user"
  service_path="$service_dir/deployctld.service"
  echo "Writing systemd user service to $service_path"
  mkdir -p "$service_dir"
  cat >"$service_path" <<EOF
[Unit]
Description=deployctl daemon
After=default.target

[Service]
Type=simple
Environment=HOME=$install_home
WorkingDirectory=$install_home
ExecStart=$PREFIX/deployctld
Restart=on-failure
RestartSec=2

[Install]
WantedBy=default.target
EOF

  systemctl --user daemon-reload
  systemctl --user enable --now deployctld.service
  systemctl --user restart deployctld.service
  echo "Service status:"
  systemctl --user --no-pager --full status deployctld.service || true
  echo "For boot-time startup on a headless server, run as root: loginctl enable-linger $install_user"
fi

echo "Installed deployctl:"
"$PREFIX/deployctl" daemon status
