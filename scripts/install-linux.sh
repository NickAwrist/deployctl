#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Install deployctl and deployctld on Linux.

Usage:
  scripts/install-linux.sh

Environment:
  PREFIX=/usr/local/bin       Binary install directory. Defaults to /usr/local/bin as root, ~/.local/bin otherwise.
  SERVICE_SCOPE=system        system or user. Defaults to system as root, user otherwise.
  DEPLOYCTL_USER=<user>       User for the systemd system service. Defaults to SUDO_USER, then current user.
  SKIP_SERVICE=1              Install binaries only; do not create/start a systemd service.

Update:
  git pull
  scripts/install-linux.sh    # or sudo scripts/install-linux.sh for a system service
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

if ! command -v go >/dev/null 2>&1; then
  echo "error: go is required to build deployctl" >&2
  exit 1
fi

if ! command -v systemctl >/dev/null 2>&1 && [[ "${SKIP_SERVICE:-0}" != "1" ]]; then
  echo "error: systemctl is required unless SKIP_SERVICE=1 is set" >&2
  exit 1
fi

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
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

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

echo "Building deployctl from $repo_root"
(
  cd "$repo_root"
  go build -trimpath -ldflags "-s -w" -o "$tmp_dir/deployctl" .
  go build -trimpath -ldflags "-s -w" -o "$tmp_dir/deployctld" ./cmd/deployctld
)

echo "Installing binaries to $PREFIX"
mkdir -p "$PREFIX"
install -m 0755 "$tmp_dir/deployctl" "$PREFIX/deployctl"
install -m 0755 "$tmp_dir/deployctld" "$PREFIX/deployctld"

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
