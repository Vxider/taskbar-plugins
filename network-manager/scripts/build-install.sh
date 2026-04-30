#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  ./network-manager/scripts/build-install.sh [options]

Options:
  --output PATH            install destination (default: ~/.local/bin/network-manager-tray)
  --no-autostart           do not install an autostart desktop entry
  --install-polkit-rule    install a pkexec rule allowing this user to run the modem helper without prompts
  --dry-run                print the resolved install plan without writing files
  -h, --help               show this help
EOF
}

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
MODULE_DIR="$(cd -- "${SCRIPT_DIR}/.." && pwd)"

INSTALL_PATH="${HOME}/.local/bin/network-manager-tray"
AUTOSTART=1
INSTALL_POLKIT=0
DRY_RUN=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output)
      if [[ $# -lt 2 ]]; then
        echo "error: --output requires a path" >&2
        exit 2
      fi
      INSTALL_PATH="$2"
      shift 2
      ;;
    --no-autostart)
      AUTOSTART=0
      shift
      ;;
    --install-polkit-rule)
      INSTALL_POLKIT=1
      shift
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if ! command -v go >/dev/null 2>&1; then
  echo "error: go is not installed or not in PATH" >&2
  exit 1
fi

BIN_DIR="$(dirname -- "${INSTALL_PATH}")"
APP_DIR="${HOME}/.local/share/applications"
AUTOSTART_DIR="${HOME}/.config/autostart"
DESKTOP_FILE="${APP_DIR}/network-manager-tray.desktop"
AUTOSTART_FILE="${AUTOSTART_DIR}/network-manager-tray.desktop"
POLKIT_RULE="/etc/polkit-1/rules.d/49-network-manager-tray.rules"

echo "==> network manager tray build + install"
echo "module: ${MODULE_DIR}"
echo "install: ${INSTALL_PATH}"
if [[ "${AUTOSTART}" -eq 1 ]]; then
  echo "autostart: enabled"
else
  echo "autostart: disabled"
fi
if [[ "${INSTALL_POLKIT}" -eq 1 ]]; then
  echo "polkit helper rule: ${POLKIT_RULE}"
fi

if [[ "${DRY_RUN}" -eq 1 ]]; then
  exit 0
fi

mkdir -p "${BIN_DIR}" "${APP_DIR}"
if [[ "${AUTOSTART}" -eq 1 ]]; then
  mkdir -p "${AUTOSTART_DIR}"
fi

TMP_BIN="$(mktemp "${TMPDIR:-/tmp}/network-manager-tray.XXXXXX")"
TMP_RULE="$(mktemp "${TMPDIR:-/tmp}/network-manager-tray-rule.XXXXXX")"
trap 'rm -f -- "${TMP_BIN}" "${TMP_RULE}"' EXIT

echo "==> building"
(
  cd "${MODULE_DIR}"
  GOPROXY="${GOPROXY:-off}" GOSUMDB="${GOSUMDB:-off}" go build -o "${TMP_BIN}" .
)

echo "==> installing"
TMP_INSTALL_PATH="${INSTALL_PATH}.new"
install -m 0755 "${TMP_BIN}" "${TMP_INSTALL_PATH}"
mv "${TMP_INSTALL_PATH}" "${INSTALL_PATH}"

cat > "${DESKTOP_FILE}" <<EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=Network Manager Tray
Comment=4G modem taskbar plugin
Exec=${INSTALL_PATH} --replace-existing
Icon=network-wireless
Terminal=false
Categories=Network;Utility;
StartupNotify=false
EOF

chmod +x "${DESKTOP_FILE}"

if [[ "${AUTOSTART}" -eq 1 ]]; then
  cp "${DESKTOP_FILE}" "${AUTOSTART_FILE}"
  chmod +x "${AUTOSTART_FILE}"
else
  rm -f "${AUTOSTART_FILE}"
fi

if [[ "${INSTALL_POLKIT}" -eq 1 ]]; then
  cat > "${TMP_RULE}" <<EOF
polkit.addRule(function(action, subject) {
  if (action.id == "org.freedesktop.policykit.exec" &&
      action.lookup("program") == "${INSTALL_PATH}" &&
      action.lookup("command_line") &&
      action.lookup("command_line").indexOf("--helper modem") >= 0 &&
      subject.user == "${USER}") {
    return polkit.Result.YES;
  }
});
EOF
  sudo install -m 0644 "${TMP_RULE}" "${POLKIT_RULE}"
fi

echo "==> done"
echo "binary: ${INSTALL_PATH}"
echo "launcher: ${DESKTOP_FILE}"
if [[ "${AUTOSTART}" -eq 1 ]]; then
  echo "autostart: ${AUTOSTART_FILE}"
fi
if [[ "${INSTALL_POLKIT}" -eq 1 ]]; then
  echo "polkit rule: ${POLKIT_RULE}"
fi
