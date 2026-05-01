#!/usr/bin/env bash
#
# PortSleuth — one-shot build & deploy script for Ubuntu 24.04 (amd64).
#
# What this does, in order:
#   1. Installs all build dependencies (Go, Node.js 20, clang, etc.)
#   2. Clones (or uses local) source
#   3. Builds the frontend (Vite → embedded into Go binary)
#   4. Generates eBPF Go bindings (clang + bpf2go)
#   5. Builds the Go binary with -tags ebpf_generated for real eBPF
#   6. Installs to /usr/local/bin/portsleuthd
#   7. Writes /etc/portsleuth/config.yaml with the port you choose
#   8. Creates a systemd unit and starts the service
#
# Usage:
#
#   # As root, from a local copy:
#   sudo PORT=8080 SOURCE_DIR=$(pwd) bash install/build-and-run.sh
#
#   # As a one-line install via curl (after the repo is on GitHub):
#   curl -fsSL https://raw.githubusercontent.com/TheMojtabam/networkmonitor/main/install/build-and-run.sh \
#     | sudo bash -s -- PORT=8080
#
# Configuration variables (env or KEY=VAL on the command line):
#   PORT          — TCP port for the web UI (default 1234)
#   SOURCE_DIR    — path to the project source. If unset, clones from GitHub.
#   REPO_URL      — git URL to clone (default: https://github.com/TheMojtabam/networkmonitor.git)
#   AUTH_ENABLED  — true|false (default false)
#   ADMIN_USER    — admin username (default admin)
#   ADMIN_PASS    — admin password (default admin)
#   GOPROXY       — Go module proxy (default "direct" — works behind firewalls)
#   GOSUMDB       — checksum database (default "off")
#   CLEAN         — set to 1 to wipe a previous installation first
#   SKIP_INSTALL  — set to 1 to build the binary but skip systemd install

set -euo pipefail

# ----- parse positional KEY=VAL arguments (so we can use `bash -s -- KEY=VAL`) -----
# This lets users pipe the script in via curl and still set env vars without
# needing to set them before the curl. Both forms work:
#
#   curl ... | sudo bash -s -- PORT=8080 AUTH_ENABLED=true
#   sudo PORT=8080 AUTH_ENABLED=true bash build-and-run.sh
#
for arg in "$@"; do
    if [[ "$arg" =~ ^[A-Z_][A-Z0-9_]*= ]]; then
        export "$arg"
    fi
done

# ----- defaults (read from env, possibly populated by the loop above) -----
PORT="${PORT:-1234}"
REPO_URL="${REPO_URL:-https://github.com/TheMojtabam/networkmonitor.git}"
SOURCE_DIR="${SOURCE_DIR:-}"
AUTH_ENABLED="${AUTH_ENABLED:-false}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASS="${ADMIN_PASS:-admin}"
SKIP_INSTALL="${SKIP_INSTALL:-0}"
CLEAN="${CLEAN:-0}"

# Go module proxy — set to "direct" by default to bypass proxies that
# may be blocked on locked-down servers. Override with GOPROXY env.
GOPROXY_VAL="${GOPROXY:-direct}"
GOSUMDB_VAL="${GOSUMDB:-off}"

INSTALL_DIR="/opt/portsleuth"
CONFIG_DIR="/etc/portsleuth"
DATA_DIR="/var/lib/portsleuth"
BIN_PATH="/usr/local/bin/portsleuthd"
SERVICE_PATH="/etc/systemd/system/portsleuth.service"
USER_NAME="portsleuth"
GROUP_NAME="portsleuth"

# Build state shared between functions
EBPF_TAG=""

# ----- helpers -----
log()  { echo -e "\033[1;34m[+]\033[0m $*" >&2; }
ok()   { echo -e "\033[1;32m[✓]\033[0m $*" >&2; }
warn() { echo -e "\033[1;33m[!]\033[0m $*" >&2; }
err()  { echo -e "\033[1;31m[✗]\033[0m $*" >&2; }

need_root() {
    if [[ ${EUID} -ne 0 ]]; then
        err "Run as root (use sudo)"
        exit 1
    fi
}

# ============================================================
# Optional cleanup of a previous half-installed instance
# ============================================================
cleanup_previous() {
    if [[ "$CLEAN" != "1" ]]; then return; fi
    log "CLEAN=1 — removing previous PortSleuth installation..."
    systemctl stop portsleuth 2>/dev/null || true
    systemctl disable portsleuth 2>/dev/null || true
    rm -f "$SERVICE_PATH" "$BIN_PATH"
    rm -rf "$INSTALL_DIR" "$DATA_DIR" "$CONFIG_DIR"
    userdel "$USER_NAME" 2>/dev/null || true
    systemctl daemon-reload || true
    rm -rf /tmp/portsleuth-build-*
    ok "Previous installation removed."
}

# ============================================================
# Go env — written to /root/.config/go/env so it persists for all
# go invocations even when sudo strips environment.
# ============================================================
configure_go_env() {
    log "Configuring Go (GOPROXY=$GOPROXY_VAL, GOSUMDB=$GOSUMDB_VAL)..."
    export GOPROXY="$GOPROXY_VAL"
    export GOSUMDB="$GOSUMDB_VAL"
    export GOFLAGS="-mod=mod"
    # Persist for any go invocation by us or by go generate.
    go env -w GOPROXY="$GOPROXY_VAL" 2>/dev/null || true
    go env -w GOSUMDB="$GOSUMDB_VAL" 2>/dev/null || true
    go env -w GOFLAGS="-mod=mod" 2>/dev/null || true
}

# ============================================================
# Step 1 — install build dependencies
# ============================================================
install_deps() {
    log "Installing build dependencies (Go, Node.js, clang, etc.)..."
    export DEBIAN_FRONTEND=noninteractive

    apt-get update -qq

    # Core build tools
    apt-get install -y -qq \
        build-essential \
        gcc-multilib \
        curl \
        git \
        unzip \
        ca-certificates \
        pkg-config \
        clang \
        llvm \
        libbpf-dev \
        linux-headers-generic \
        linux-tools-common \
        iproute2 \
        2>&1 | grep -v '^$' || true

    # Go 1.23 — Ubuntu 24 has it in the archive
    if ! command -v go >/dev/null 2>&1 || [[ "$(go version | awk '{print $3}')" < "go1.22" ]]; then
        log "Installing Go 1.23..."
        apt-get install -y -qq golang-1.23 || apt-get install -y -qq golang
        # golang-1.23 installs to /usr/lib/go-1.23/bin
        if [[ -x /usr/lib/go-1.23/bin/go ]]; then
            ln -sf /usr/lib/go-1.23/bin/go /usr/local/bin/go
        fi
    fi

    # Node.js 20 — Ubuntu 24's default may be older; install via NodeSource if needed
    if ! command -v node >/dev/null 2>&1 || [[ "$(node --version | sed 's/v//' | cut -d. -f1)" -lt 20 ]]; then
        log "Installing Node.js 20..."
        curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
        apt-get install -y -qq nodejs
    fi

    ok "Dependencies installed."
    log "  go:    $(go version 2>/dev/null || echo 'NOT FOUND')"
    log "  node:  $(node --version 2>/dev/null || echo 'NOT FOUND')"
    log "  clang: $(clang --version 2>/dev/null | head -1 || echo 'NOT FOUND')"
}

# ============================================================
# Step 2 — get source
# ============================================================
get_source() {
    if [[ -n "$SOURCE_DIR" ]]; then
        if [[ ! -d "$SOURCE_DIR/backend" ]]; then
            err "SOURCE_DIR=$SOURCE_DIR doesn't look like the project (no backend/ dir)"
            exit 2
        fi
        log "Using local source: $SOURCE_DIR"
        WORK_DIR="$SOURCE_DIR"
        return
    fi

    WORK_DIR="/tmp/portsleuth-build-$$"
    log "Cloning $REPO_URL → $WORK_DIR"
    git clone --depth 1 "$REPO_URL" "$WORK_DIR"
}

# ============================================================
# Step 3 — build frontend
# ============================================================
build_frontend() {
    log "Building frontend..."
    cd "$WORK_DIR/frontend"

    # Use ci if lockfile exists, else install
    if [[ -f package-lock.json ]]; then
        npm ci --silent --no-audit --no-fund
    else
        npm install --silent --no-audit --no-fund
    fi

    npm run build

    if [[ ! -f "$WORK_DIR/backend/cmd/portsleuthd/web/index.html" ]]; then
        err "Frontend build did not produce index.html in the expected location"
        exit 3
    fi

    ok "Frontend built."
}

# ============================================================
# Step 4 — generate eBPF bindings
# ============================================================
generate_ebpf() {
    log "Generating eBPF Go bindings..."
    cd "$WORK_DIR/backend"

    GOPROXY="$GOPROXY_VAL" GOSUMDB="$GOSUMDB_VAL" go mod download

    cd "$WORK_DIR/backend/internal/collector/ebpf"
    if GOPROXY="$GOPROXY_VAL" GOSUMDB="$GOSUMDB_VAL" go generate ./... 2>&1; then
        ok "eBPF bindings generated."
        EBPF_TAG="ebpf_generated"
    else
        warn "eBPF generation failed — building without real eBPF (the ss-based fallback will be used)."
        warn "(If you want real eBPF later: ensure clang + libbpf-dev + linux-headers + gcc-multilib are installed and re-run with CLEAN=1)"
        EBPF_TAG=""
    fi
}

# ============================================================
# Step 5 — build the Go binary
# ============================================================
build_backend() {
    log "Building Go backend..."
    cd "$WORK_DIR/backend"

    mkdir -p "$WORK_DIR/bin"

    # Use array form so empty $EBPF_TAG doesn't add an empty arg.
    local build_args=()
    if [[ -n "$EBPF_TAG" ]]; then
        build_args+=("-tags" "ebpf_generated")
        log "  building with real eBPF support"
    else
        log "  building with /proc + ss fallback (no real eBPF)"
    fi
    build_args+=("-ldflags=-s -w")
    build_args+=("-o" "$WORK_DIR/bin/portsleuthd")
    build_args+=("./cmd/portsleuthd")

    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
        GOPROXY="$GOPROXY_VAL" GOSUMDB="$GOSUMDB_VAL" \
        go build "${build_args[@]}"

    if [[ ! -x "$WORK_DIR/bin/portsleuthd" ]]; then
        err "Build did not produce a binary"
        exit 4
    fi

    SIZE=$(du -h "$WORK_DIR/bin/portsleuthd" | awk '{print $1}')
    ok "Built $WORK_DIR/bin/portsleuthd ($SIZE)"
}

# ============================================================
# Step 6 — create user, install binary, write config, install systemd
# ============================================================
install_service() {
    if [[ "$SKIP_INSTALL" == "1" ]]; then
        log "SKIP_INSTALL=1 — leaving binary at $WORK_DIR/bin/portsleuthd"
        return
    fi

    log "Stopping existing service (if any)..."
    if systemctl list-unit-files portsleuth.service >/dev/null 2>&1; then
        systemctl stop portsleuth.service 2>/dev/null || true
    fi

    log "Creating user '$USER_NAME'..."
    if ! id -u "$USER_NAME" >/dev/null 2>&1; then
        useradd --system --home "$INSTALL_DIR" --shell /usr/sbin/nologin "$USER_NAME"
    fi

    log "Creating directories..."
    mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$CONFIG_DIR"
    chown -R "$USER_NAME:$GROUP_NAME" "$INSTALL_DIR" "$DATA_DIR"
    chown root:"$GROUP_NAME" "$CONFIG_DIR"
    chmod 0750 "$CONFIG_DIR"

    log "Installing binary to $BIN_PATH..."
    install -m 0755 "$WORK_DIR/bin/portsleuthd" "$BIN_PATH"

    log "Writing config to $CONFIG_DIR/config.yaml..."
    JWT_SECRET=$(head -c 48 /dev/urandom | base64 | tr -d '/+=' | head -c 32)
    cat > "$CONFIG_DIR/config.yaml" <<EOFCFG
# PortSleuth configuration
# Generated by build-and-run.sh on $(date -Iseconds)

server:
  listen: ":${PORT}"
  read_timeout_sec: 15
  write_timeout_sec: 15

collector:
  interface_interval_ms: 1000
  port_interval_ms: 5000
  ebpf_enabled: true
  ebpf_interfaces: ["auto"]

storage:
  memory_window_hours: 24
  sqlite_path: "${DATA_DIR}/history.db"
  retention_days: 30

auth:
  enabled: ${AUTH_ENABLED}
  jwt_secret: "${JWT_SECRET}"
  admin_username: "${ADMIN_USER}"
  admin_password: "${ADMIN_PASS}"
  session_hours: 24

geoip:
  enabled: false
  country_db: ""
  asn_db: ""

alerts:
  enabled: true
  rules_file: "${CONFIG_DIR}/alerts.yaml"

prometheus:
  enabled: false
  path: "/metrics"

logging:
  level: "info"
  format: "text"
EOFCFG
    chmod 0640 "$CONFIG_DIR/config.yaml"
    chown root:"$GROUP_NAME" "$CONFIG_DIR/config.yaml"

    log "Installing systemd unit..."
    cat > "$SERVICE_PATH" <<EOFUNIT
[Unit]
Description=PortSleuth (network & system monitoring)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$USER_NAME
Group=$GROUP_NAME
WorkingDirectory=$INSTALL_DIR
ExecStart=$BIN_PATH --config $CONFIG_DIR/config.yaml
Restart=always
RestartSec=2

# eBPF + raw network access
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

    log "Starting service..."
    systemctl daemon-reload
    systemctl enable portsleuth.service
    systemctl start portsleuth.service

    sleep 2
    if systemctl is-active --quiet portsleuth.service; then
        ok "Service is running."
    else
        err "Service failed to start. Check: journalctl -u portsleuth -n 50"
        exit 5
    fi
}

# ============================================================
# Step 7 — smoke-test the API
# ============================================================
smoke_test() {
    if [[ "$SKIP_INSTALL" == "1" ]]; then return; fi

    log "Smoke-testing the API..."
    sleep 1
    if curl -fsS "http://127.0.0.1:${PORT}/api/health" >/dev/null 2>&1; then
        ok "API is responding on port $PORT"
    else
        warn "API not responding yet (service may still be starting)"
    fi
}

# ============================================================
# Step 8 — print summary
# ============================================================
print_summary() {
    cat <<EOF

========================================================================
  PortSleuth installed successfully.
========================================================================

  URL:       http://$(hostname -I | awk '{print $1}'):${PORT}
             (or http://localhost:${PORT} if you're SSH-tunneled)

  Auth:      $([ "$AUTH_ENABLED" = "true" ] && echo "ENABLED — login as ${ADMIN_USER} / ${ADMIN_PASS}" || echo "disabled (no login required)")

  Config:    $CONFIG_DIR/config.yaml
  Data:      $DATA_DIR
  Binary:    $BIN_PATH
  Logs:      journalctl -u portsleuth -f

  Useful commands:
    systemctl status portsleuth        # service status
    systemctl restart portsleuth       # restart after config change
    journalctl -u portsleuth -f        # tail logs
    curl http://localhost:${PORT}/api/health
    curl http://localhost:${PORT}/api/snapshot | jq

  Open the UI in your browser to see live network monitoring.
========================================================================

EOF
}

# ============================================================
# Main
# ============================================================
main() {
    need_root

    log "PortSleuth build & deploy starting (PORT=$PORT)"

    cleanup_previous
    install_deps
    configure_go_env
    get_source
    build_frontend
    generate_ebpf
    build_backend
    install_service
    smoke_test
    print_summary
}

main "$@"
