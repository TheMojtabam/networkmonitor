#!/usr/bin/env bash
set -euo pipefail

# PortSleuth installer
# One-liner usage:
#   curl -fsSL https://raw.githubusercontent.com/TheMojtabam/networkmonitor/main/install/install.sh | sudo bash -s install
#   curl -fsSL https://raw.githubusercontent.com/TheMojtabam/networkmonitor/main/install/install.sh | sudo bash -s update
#   curl -fsSL https://raw.githubusercontent.com/TheMojtabam/networkmonitor/main/install/install.sh | sudo bash -s uninstall

REPO="TheMojtabam/networkmonitor"
APP="portsleuth"
USER="portsleuth"
GROUP="portsleuth"
INSTALL_DIR="/opt/portsleuth"
BIN_DIR="/usr/local/bin"
SERVICE="/etc/systemd/system/portsleuth.service"
PORT=${PORTSLEUTH_PORT:-1234}

usage(){ echo "Usage: $0 {install|update|uninstall}"; }
need_root(){ [[ ${EUID} -eq 0 ]] || { echo "Run as root" >&2; exit 1; }; }

ensure_user(){
  if ! id -u "$USER" >/dev/null 2>&1; then
    useradd --system --home "$INSTALL_DIR" --shell /usr/sbin/nologin "$USER"
  fi
}

arch(){
  local m
  m=$(uname -m)
  case "$m" in
    x86_64|amd64) echo "amd64";;
    aarch64|arm64) echo "arm64";;
    *) echo "$m";;
  esac
}

os(){
  local s
  s=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$s" in
    linux) echo "linux";;
    *) echo "$s";;
  esac
}

latest_tag(){
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1
}

download_release(){
  local tag os_ arch_ url tmp
  os_=$(os)
  arch_=$(arch)
  tag=$(latest_tag)
  if [[ -z "$tag" ]]; then
    echo "No GitHub releases found for ${REPO}. Create a release first." >&2
    exit 2
  fi

  url="https://github.com/${REPO}/releases/download/${tag}/portsleuthd_${os_}_${arch_}"
  tmp=$(mktemp)
  echo "Downloading: $url" >&2
  curl -fL "$url" -o "$tmp"
  install -m 0755 "$tmp" "${BIN_DIR}/portsleuthd"
  rm -f "$tmp"
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

# For eBPF mode (when implemented) these capabilities matter:
AmbientCapabilities=CAP_NET_ADMIN CAP_BPF CAP_PERFMON
CapabilityBoundingSet=CAP_NET_ADMIN CAP_BPF CAP_PERFMON
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable --now portsleuth.service
}

cmd_install(){
  need_root
  mkdir -p "$INSTALL_DIR"
  ensure_user
  chown -R "$USER:$GROUP" "$INSTALL_DIR"

  download_release
  install_unit
  echo "Installed. Open: http://<SERVER_IP>:${PORT}/" >&2
}

cmd_update(){
  need_root
  systemctl stop portsleuth.service || true
  download_release
  systemctl start portsleuth.service || true
  echo "Updated." >&2
}

cmd_uninstall(){
  need_root
  systemctl disable --now portsleuth.service || true
  rm -f "$SERVICE"
  systemctl daemon-reload || true
  rm -f "$BIN_DIR/portsleuthd" || true
  rm -rf "$INSTALL_DIR" || true
  userdel "$USER" 2>/dev/null || true
  echo "Uninstalled." >&2
}

case "${1:-}" in
  install) cmd_install ;;
  update) cmd_update ;;
  uninstall) cmd_uninstall ;;
  *) usage; exit 1 ;;
esac
