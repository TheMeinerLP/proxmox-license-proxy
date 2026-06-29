#!/bin/sh
# proxmox-license-proxy installer — detects CPU arch + package format and
# installs the matching .deb / .rpm / .apk from the latest GitHub release.
#
#   curl -fsSL https://raw.githubusercontent.com/TheMeinerLP/proxmox-license-proxy/master/install.sh | sh
#   VERSION=0.2.0 sh install.sh        # pin a specific version
#
# For private / internal lab use only — see the README warning.
set -eu

REPO="TheMeinerLP/proxmox-license-proxy"
BIN="proxmox-license-proxy"

info() { echo ">> $*"; }
err() {
	echo "error: $*" >&2
	exit 1
}

# --- privilege escalation -------------------------------------------------
if [ "$(id -u)" -eq 0 ]; then
	SUDO=""
elif command -v sudo >/dev/null 2>&1; then
	SUDO="sudo"
else
	err "need root privileges: run as root or install sudo"
fi

# --- need a downloader ----------------------------------------------------
if command -v curl >/dev/null 2>&1; then
	HAVE_CURL=1
elif command -v wget >/dev/null 2>&1; then
	HAVE_CURL=0
else
	err "need curl or wget"
fi

download() { # download <url> <dest>
	if [ "$HAVE_CURL" -eq 1 ]; then
		curl -fsSL -o "$2" "$1"
	else
		wget -qO "$2" "$1"
	fi
}

# --- detect CPU architecture ----------------------------------------------
case "$(uname -m)" in
x86_64 | amd64) ARCH=amd64 ;;
aarch64 | arm64) ARCH=arm64 ;;
*) err "unsupported architecture: $(uname -m) (only amd64 and arm64 are built)" ;;
esac

# --- detect package format from available tooling -------------------------
if command -v apt-get >/dev/null 2>&1 || command -v dpkg >/dev/null 2>&1; then
	FMT=deb
elif command -v dnf >/dev/null 2>&1 || command -v yum >/dev/null 2>&1 || command -v rpm >/dev/null 2>&1; then
	FMT=rpm
elif command -v apk >/dev/null 2>&1; then
	FMT=apk
else
	err "no supported package manager found (need apt/dpkg, dnf/yum/rpm, or apk)"
fi

# --- resolve version (latest unless VERSION is set) -----------------------
VERSION="${VERSION:-}"
if [ -z "$VERSION" ]; then
	info "resolving latest release..."
	if [ "$HAVE_CURL" -eq 1 ]; then
		TAG=$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
			"https://github.com/$REPO/releases/latest" | sed 's#.*/##')
	else
		TAG=$(wget -q -S -O /dev/null "https://github.com/$REPO/releases/latest" 2>&1 |
			sed -n 's#.*[Ll]ocation:.*tag/\(.*\)#\1#p' | tr -d '\r' | tail -1)
	fi
	[ -n "$TAG" ] || err "could not determine latest version; set VERSION=x.y.z"
	VERSION="${TAG#v}"
else
	VERSION="${VERSION#v}"
	TAG="v${VERSION}"
fi

FILE="${BIN}_${VERSION}_linux_${ARCH}.${FMT}"
URL="https://github.com/$REPO/releases/download/${TAG}/${FILE}"

# --- download -------------------------------------------------------------
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM

info "downloading ${FILE} (${TAG})"
download "$URL" "$TMP/$FILE" || err "download failed: $URL"

# --- install --------------------------------------------------------------
info "installing via ${FMT}"
case "$FMT" in
deb)
	if command -v apt-get >/dev/null 2>&1; then
		$SUDO apt-get install -y "$TMP/$FILE"
	else
		$SUDO dpkg -i "$TMP/$FILE"
	fi
	;;
rpm)
	if command -v dnf >/dev/null 2>&1; then
		$SUDO dnf install -y "$TMP/$FILE"
	elif command -v yum >/dev/null 2>&1; then
		$SUDO yum install -y "$TMP/$FILE"
	else
		$SUDO rpm -Uvh "$TMP/$FILE"
	fi
	;;
apk)
	$SUDO apk add --allow-untrusted "$TMP/$FILE"
	;;
esac

info "installed: $("$BIN" version 2>/dev/null | head -1 || echo "$BIN $VERSION")"
echo
info "next steps:"
echo "   1. review the config:  ${SUDO:+$SUDO }\$EDITOR /etc/pmox/config.yaml"
echo "   2. start the service:  ${SUDO:+$SUDO }systemctl start ${BIN}"
echo "   3. add a license:      ${BIN} license add pbsc-1234567890"
