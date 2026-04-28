#!/usr/bin/env bash
set -euo pipefail

# PortSleuth installer
# Supports: install | update | uninstall

APP=portsleuth
USER=portsleuth
GROUP=portsleuth
INSTALL_DIR=/opt/portsleuth
BIN_DIR=/usr/local/bin
SERVICE=/etc/systemd/system/portsleuth.service
PORT=${PORTSLEUTH_PORT:-1234}

usage(){
  echo "Usage: $0 {install|update|uninstall}"
}

need_root(){
  if [[ ${EUID} -ne 0 ]]; then
    echo "Run as root" >&2
    exit 1
  fi
}

ensure_user(){
  if ! id -u "$USER" >/dev/null 2>&1; then
    useradd --system --home "$INSTALL_DIR" --shell /usr/sbin/nologin "$USER"
  fi
}

install_unit(){
  cat > "$SERVICE" <<EOF
[Unit]
Description=PortSleuth (network & system monitoring)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$USER
Group=$GROUP
WorkingDirectory=$INSTALL_DIR
Environment=PORTSLEUTH_PORT=$PORT
ExecStart=$BIN_DIR/portsleuthd --listen :$PORT
Restart=always
RestartSec=2
AmbientCapabilities=CAP_NET_ADMIN CAP_BPF CAP_PERFMON
CapabilityBoundingSet=CAP_NET_ADMIN CAP_BPF CAP_PERFMON
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable --now portsleuth.service
}

download_release(){
  # TODO: replace with real GitHub releases URL once repo exists.
  echo "TODO: download release (not wired yet)." >&2
  echo "For now, build from source." >&2
  exit 2
}

cmd_install(){
  need_root
  mkdir -p "$INSTALL_DIR"
  ensure_user
  chown -R "$USER:$GROUP" "$INSTALL_DIR"

  download_release
  install_unit
}

cmd_update(){
  need_root
  systemctl stop portsleuth.service || true
  download_release
  systemctl start portsleuth.service || true
}

cmd_uninstall(){
  need_root
  systemctl disable --now portsleuth.service || true
  rm -f "$SERVICE"
  systemctl daemon-reload || true
  rm -f "$BIN_DIR/portsleuthd" || true
  rm -rf "$INSTALL_DIR" || true
  userdel "$USER" 2>/dev/null || true
}

case "${1:-}" in
  install) cmd_install ;;
  update) cmd_update ;;
  uninstall) cmd_uninstall ;;
  *) usage; exit 1 ;;
esac
