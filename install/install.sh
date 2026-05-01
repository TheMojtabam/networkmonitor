#!/usr/bin/env bash
set -euo pipefail

# PortSleuth installer — single-file install/update/uninstall
#
# One-liner usage:
#   curl -fsSL https://raw.githubusercontent.com/TheMojtabam/networkmonitor/main/install/install.sh | sudo bash -s install
#   curl -fsSL https://raw.githubusercontent.com/TheMojtabam/networkmonitor/main/install/install.sh | sudo bash -s update
#   curl -fsSL https://raw.githubusercontent.com/TheMojtabam/networkmonitor/main/install/install.sh | sudo bash -s uninstall

REPO="TheMojtabam/networkmonitor"
APP="portsleuth"
USER="portsleuth"
GROUP="portsleuth"
INSTALL_DIR="/opt/portsleuth"
CONFIG_DIR="/etc/portsleuth"
DATA_DIR="/var/lib/portsleuth"
BIN_DIR="/usr/local/bin"
SERVICE="/etc/systemd/system/portsleuth.service"
PORT="${PORTSLEUTH_PORT:-1234}"

usage(){ echo "Usage: $0 {install|update|uninstall}"; }
need_root(){ [[ ${EUID} -eq 0 ]] || { echo "Run as root" >&2; exit 1; }; }

has_systemd(){
  command -v systemctl >/dev/null 2>&1 && systemctl is-system-running >/dev/null 2>&1
}

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
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' \
    | head -n1
}

# ---------------------------------------------------------------
# Service control helpers — IDEMPOTENT (safe to call multiple times)
# ---------------------------------------------------------------

stop_service(){
  # Stop running portsleuth no matter how it was started.
  # This is the FIX for the install bug: download_release would otherwise
  # try to overwrite a binary that the running process still holds open
  # (and which is still bound to the listen port).
  if has_systemd; then
    if systemctl list-unit-files portsleuth.service >/dev/null 2>&1; then
      systemctl stop portsleuth.service 2>/dev/null || true
    fi
  fi
  stop_no_systemd
}

start_service(){
  if has_systemd; then
    systemctl start portsleuth.service || true
  else
    start_no_systemd
  fi
}

start_no_systemd(){
  mkdir -p "$INSTALL_DIR/run" "$INSTALL_DIR/log"
  local pidfile="$INSTALL_DIR/run/portsleuth.pid"
  if [[ -f "$pidfile" ]] && kill -0 "$(cat "$pidfile")" 2>/dev/null; then
    echo "Already running (pid $(cat "$pidfile"))." >&2
    return 0
  fi
  nohup "$BIN_DIR/portsleuthd" \
    --config "$CONFIG_DIR/config.yaml" \
    >"$INSTALL_DIR/log/portsleuth.log" 2>&1 &
  echo $! > "$pidfile"
  echo "Started (no systemd). pid=$(cat "$pidfile")" >&2
}

stop_no_systemd(){
  local pidfile="$INSTALL_DIR/run/portsleuth.pid"
  if [[ -f "$pidfile" ]]; then
    local pid
    pid="$(cat "$pidfile" 2>/dev/null || echo)"
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      # wait up to 5s for graceful shutdown
      for _ in 1 2 3 4 5; do
        kill -0 "$pid" 2>/dev/null || break
        sleep 1
      done
      kill -9 "$pid" 2>/dev/null || true
    fi
    rm -f "$pidfile"
  fi
}

# ---------------------------------------------------------------
# Download & install
# ---------------------------------------------------------------

download_release(){
  local tag os_ arch_ url tmp
  os_=$(os)
  arch_=$(arch)
  tag=$(latest_tag)
  if [[ -z "$tag" ]]; then
    echo "No GitHub releases found for ${REPO}." >&2
    exit 2
  fi

  url="https://github.com/${REPO}/releases/download/${tag}/portsleuthd_${os_}_${arch_}"
  tmp=$(mktemp)
  echo "Downloading ${tag}: $url" >&2
  curl -fL "$url" -o "$tmp"
  install -m 0755 "$tmp" "${BIN_DIR}/portsleuthd"
  rm -f "$tmp"
}

write_default_config(){
  # Only write if config doesn't exist; never overwrite user config.
  if [[ -f "$CONFIG_DIR/config.yaml" ]]; then
    return 0
  fi
  mkdir -p "$CONFIG_DIR"
  cat > "$CONFIG_DIR/config.yaml" <<EOFCFG
# PortSleuth configuration

server:
  listen: ":${PORT}"
  read_timeout_sec: 15
  write_timeout_sec: 15

collector:
  # interval in milliseconds for interface and per-port sampling
  interface_interval_ms: 1000
  port_interval_ms: 5000
  # use eBPF for accurate per-port byte counters
  ebpf_enabled: true
  # interfaces to attach eBPF to ("auto" = all non-loopback)
  ebpf_interfaces: ["auto"]

storage:
  # in-memory ring buffer window
  memory_window_hours: 24
  # SQLite path for long-term aggregates ("" disables persistence)
  sqlite_path: "${DATA_DIR}/history.db"
  retention_days: 30

auth:
  # set to false to disable login (use for trusted networks only)
  enabled: false
  # JWT signing secret — change this!
  jwt_secret: "CHANGE_ME_TO_A_LONG_RANDOM_STRING"
  # default admin credentials, applied on first start only
  admin_username: "admin"
  admin_password: "admin"
  session_hours: 24

geoip:
  enabled: false
  # path to MaxMind GeoLite2-Country.mmdb and GeoLite2-ASN.mmdb
  country_db: ""
  asn_db: ""

alerts:
  enabled: true
  # rules can also be managed via the UI; UI changes are saved to alerts.yaml
  rules_file: "${CONFIG_DIR}/alerts.yaml"

prometheus:
  enabled: false
  path: "/metrics"

logging:
  level: "info"
  format: "text"
EOFCFG
  chmod 0640 "$CONFIG_DIR/config.yaml"
}

install_unit(){
  cat > "$SERVICE" <<EOFUNIT
[Unit]
Description=PortSleuth (network & system monitoring)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$USER
Group=$GROUP
WorkingDirectory=$INSTALL_DIR
ExecStart=$BIN_DIR/portsleuthd --config $CONFIG_DIR/config.yaml
Restart=always
RestartSec=2

# eBPF needs these
AmbientCapabilities=CAP_NET_ADMIN CAP_BPF CAP_PERFMON CAP_SYS_RESOURCE
CapabilityBoundingSet=CAP_NET_ADMIN CAP_BPF CAP_PERFMON CAP_SYS_RESOURCE
NoNewPrivileges=true

# Hardening
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=$DATA_DIR $INSTALL_DIR

[Install]
WantedBy=multi-user.target
EOFUNIT

  systemctl daemon-reload
  systemctl enable portsleuth.service
}

# ---------------------------------------------------------------
# Top-level commands
# ---------------------------------------------------------------

cmd_install(){
  need_root

  # FIX: stop any existing portsleuth before overwriting the binary.
  # Previously cmd_install skipped this and the running process kept
  # the listen port bound + held the old binary in memory — the new
  # version "installed" but never actually came up.
  echo "Stopping any existing PortSleuth instance..." >&2
  stop_service

  mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$CONFIG_DIR"
  ensure_user
  chown -R "$USER:$GROUP" "$INSTALL_DIR" "$DATA_DIR"
  chown root:"$GROUP" "$CONFIG_DIR"
  chmod 0750 "$CONFIG_DIR"

  download_release
  write_default_config

  if has_systemd; then
    install_unit
    systemctl start portsleuth.service
  else
    start_no_systemd
  fi

  echo "" >&2
  echo "PortSleuth installed." >&2
  echo "  Config:  $CONFIG_DIR/config.yaml" >&2
  echo "  Data:    $DATA_DIR" >&2
  echo "  Binary:  $BIN_DIR/portsleuthd" >&2
  echo "  Open:    http://<SERVER_IP>:${PORT}" >&2
}

cmd_update(){
  need_root
  stop_service
  download_release
  start_service
  echo "Updated." >&2
}

cmd_uninstall(){
  need_root
  stop_service
  if has_systemd && [[ -f "$SERVICE" ]]; then
    systemctl disable portsleuth.service 2>/dev/null || true
    rm -f "$SERVICE"
    systemctl daemon-reload || true
  fi
  rm -f "$BIN_DIR/portsleuthd"
  rm -rf "$INSTALL_DIR"
  # Keep $CONFIG_DIR and $DATA_DIR by default — uncomment to wipe:
  # rm -rf "$CONFIG_DIR" "$DATA_DIR"
  userdel "$USER" 2>/dev/null || true
  echo "Uninstalled. Config and data preserved at $CONFIG_DIR and $DATA_DIR." >&2
}

case "${1:-}" in
  install)   cmd_install ;;
  update)    cmd_update ;;
  uninstall) cmd_uninstall ;;
  *)         usage; exit 1 ;;
esac
