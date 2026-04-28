#!/usr/bin/env bash
set -euo pipefail

REPO="TheMojtabam/networkmonitor"
BASE_URL="https://raw.githubusercontent.com/${REPO}/main/install"
INSTALL_SH_URL="${BASE_URL}/install.sh"

bold() { printf "\033[1m%s\033[0m\n" "$*"; }

run_action() {
  local action="$1"
  curl -fsSL "$INSTALL_SH_URL" | sudo bash -s "$action"
}

clear || true
bold "PortSleuth Installer"
echo "Repo: ${REPO}"
echo

echo "1) Install"
echo "2) Update"
echo "3) Uninstall"
echo "4) Exit"
echo
read -rp "Select: " choice

case "${choice}" in
  1) run_action install ;;
  2) run_action update ;;
  3) run_action uninstall ;;
  4) exit 0 ;;
  *) echo "Invalid choice"; exit 1 ;;
esac
