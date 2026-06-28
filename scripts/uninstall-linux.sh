#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Uninstall deployctl from Linux.

Usage:
  scripts/uninstall-linux.sh

Environment:
  PREFIX=/usr/local/bin       Binary install directory. Defaults to /usr/local/bin as root, ~/.local/bin otherwise.
  SERVICE_SCOPE=system        system or user. Defaults to system as root, user otherwise.
  REMOVE_DATA=1               Also remove ~/.deployctl for the service user/current user.
  DEPLOYCTL_USER=<user>       User whose ~/.deployctl should be removed when REMOVE_DATA=1 and SERVICE_SCOPE=system.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "error: uninstall-linux.sh only supports Linux" >&2
  exit 1
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

if [[ -z "${PREFIX:-}" ]]; then
  if [[ "$SERVICE_SCOPE" == "system" ]]; then
    PREFIX="/usr/local/bin"
  else
    PREFIX="$HOME/.local/bin"
  fi
fi

if [[ "$SERVICE_SCOPE" == "system" ]]; then
  if [[ "$is_root" != "1" ]]; then
    echo "error: SERVICE_SCOPE=system requires root. Re-run with sudo, or use SERVICE_SCOPE=user." >&2
    exit 1
  fi
  systemctl disable --now deployctld.service >/dev/null 2>&1 || true
  rm -f /etc/systemd/system/deployctld.service
  systemctl daemon-reload
else
  systemctl --user disable --now deployctld.service >/dev/null 2>&1 || true
  rm -f "$HOME/.config/systemd/user/deployctld.service"
  systemctl --user daemon-reload
fi

rm -f "$PREFIX/deployctl" "$PREFIX/deployctld"

if [[ "${REMOVE_DATA:-0}" == "1" ]]; then
  if [[ "$SERVICE_SCOPE" == "system" ]]; then
    install_user="${DEPLOYCTL_USER:-${SUDO_USER:-$(id -un)}}"
    install_home="$(getent passwd "$install_user" | cut -d: -f6)"
  else
    install_home="$HOME"
  fi
  rm -rf "$install_home/.deployctl"
fi

echo "deployctl uninstalled"
