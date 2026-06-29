#!/bin/sh
# proxmox-license-proxy installer.
#
# Default: detect CPU arch + package format and install the matching
# .deb / .rpm / .apk from the latest GitHub release (ships the systemd service).
#
#   curl -fsSL https://raw.githubusercontent.com/TheMeinerLP/proxmox-license-proxy/main/install.sh | sh
#
# CLI-only: install just the binary + shell completions from the release
# tarball, with NO systemd service and NO config (for running the CLI / `serve`
# by hand, e.g. on a single host):
#
#   curl -fsSL .../install.sh | PMOX_CLI_ONLY=1 sh
#   curl -fsSL .../install.sh | sh -s -- --cli-only
#
# Pin a version with PMOX_VERSION (set it on the `sh`, not the `curl`, when
# piping):
#
#   curl -fsSL .../install.sh | PMOX_VERSION=2.0.0 sh
#
# Override the CLI-only binary dir with PMOX_BINDIR. PMOX_VERSION/PMOX_BINDIR
# also accept the legacy bare VERSION/BINDIR names. Private / lab use only.
set -eu

REPO="TheMeinerLP/proxmox-license-proxy"
BIN="proxmox-license-proxy"
CLI_ONLY="${PMOX_CLI_ONLY:-0}"
# Honour the PMOX_* convention; keep the bare names as a back-compat fallback.
BINDIR="${PMOX_BINDIR:-${BINDIR:-/usr/local/bin}}"
PIN_VERSION="${PMOX_VERSION:-${VERSION:-}}"

for arg in "$@"; do
	case "$arg" in
	--cli-only) CLI_ONLY=1 ;;
	esac
done

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

# --- detect package format from available tooling (package mode only) -----
if [ "$CLI_ONLY" -ne 1 ]; then
	if command -v apt-get >/dev/null 2>&1 || command -v dpkg >/dev/null 2>&1; then
		FMT=deb
	elif command -v dnf >/dev/null 2>&1 || command -v yum >/dev/null 2>&1 || command -v rpm >/dev/null 2>&1; then
		FMT=rpm
	elif command -v apk >/dev/null 2>&1; then
		FMT=apk
	else
		err "no supported package manager found; retry with --cli-only to install just the binary"
	fi
fi

# --- resolve version (latest unless PMOX_VERSION / VERSION is set) ---------
if [ -z "$PIN_VERSION" ]; then
	info "resolving latest release..."
	if [ "$HAVE_CURL" -eq 1 ]; then
		TAG=$(curl -fsSLI -o /dev/null -w '%{url_effective}' \
			"https://github.com/$REPO/releases/latest" | sed 's#.*/##')
	else
		TAG=$(wget -q -S -O /dev/null "https://github.com/$REPO/releases/latest" 2>&1 |
			sed -n 's#.*[Ll]ocation:.*tag/\(.*\)#\1#p' | tr -d '\r' | tail -1)
	fi
	[ -n "$TAG" ] || err "could not determine latest version; set PMOX_VERSION=x.y.z"
	VERSION="${TAG#v}"
else
	VERSION="${PIN_VERSION#v}"
	TAG="v${VERSION}"
	info "pinned to ${TAG}"
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM

# --- CLI-only: binary + completions from the tarball, no service ----------
if [ "$CLI_ONLY" -eq 1 ]; then
	FILE="${BIN}_${VERSION}_linux_${ARCH}.tar.gz"
	URL="https://github.com/$REPO/releases/download/${TAG}/${FILE}"
	info "downloading ${FILE} (${TAG})"
	download "$URL" "$TMP/$FILE" || err "download failed: $URL"
	tar -xzf "$TMP/$FILE" -C "$TMP" || err "could not extract $FILE"

	info "installing binary to ${BINDIR}/${BIN}"
	$SUDO install -d "$BINDIR"
	$SUDO install -m 0755 "$TMP/$BIN" "$BINDIR/$BIN"

	# Shell completions, each into its system dir if that dir exists.
	install_comp() { # install_comp <src> <dst>
		[ -f "$1" ] || return 0
		dst_dir=$(dirname "$2")
		[ -d "$dst_dir" ] || return 0
		$SUDO install -m 0644 "$1" "$2" && info "completion: $2"
	}
	install_comp "$TMP/completions/${BIN}.bash" "/usr/share/bash-completion/completions/${BIN}"
	install_comp "$TMP/completions/${BIN}.zsh" "/usr/share/zsh/vendor-completions/_${BIN}"
	install_comp "$TMP/completions/${BIN}.fish" "/usr/share/fish/vendor_completions.d/${BIN}.fish"

	info "installed: $("$BINDIR/$BIN" version 2>/dev/null | head -1 || echo "$BIN $VERSION")"
	echo
	info "next steps (no service installed):"
	echo "   1. run the server:      ${BIN} serve   (scaffold a config: ${BIN} config init)"
	echo "   2. on each Proxmox host: ${BIN} client enroll --server https://<this-host>"
	echo "      approve it here:      ${BIN} account approve <thumbprint>"
	echo "      (or mint a key by hand: ${BIN} subscription generate)"
	exit 0
fi

# --- package install (deb/rpm/apk), ships the systemd service -------------
FILE="${BIN}_${VERSION}_linux_${ARCH}.${FMT}"
URL="https://github.com/$REPO/releases/download/${TAG}/${FILE}"

info "downloading ${FILE} (${TAG})"
download "$URL" "$TMP/$FILE" || err "download failed: $URL"

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

# Report the version from the package's own binary (/usr/bin), not from PATH:
# an older 'client install' copy in /usr/local/bin would otherwise shadow it and
# report the wrong version.
PKG_BIN="/usr/bin/${BIN}"
info "installed: $("$PKG_BIN" version 2>/dev/null | head -1 || echo "$BIN $VERSION")"

# Warn loudly if a different binary wins in PATH (the classic shadowing footgun).
RESOLVED=$(command -v "$BIN" 2>/dev/null || true)
if [ -n "$RESOLVED" ] && [ "$RESOLVED" != "$PKG_BIN" ]; then
	echo
	echo "   WARNING: '${BIN}' in your PATH is ${RESOLVED}, not the package at ${PKG_BIN}."
	echo "            It shadows the installed package (likely an old 'client install')."
	echo "            Fix: rm -f ${RESOLVED} && hash -r"
fi

echo
info "next steps:"
echo "   1. review the config:    ${SUDO:+$SUDO }\$EDITOR /etc/pmox/config.yaml"
echo "   2. start the service:    ${SUDO:+$SUDO }systemctl start ${BIN}"
echo "   3. on each Proxmox host: ${BIN} client enroll --server https://<this-host>"
echo "      approve it here:      ${PKG_BIN} account approve <thumbprint>"
echo "      (or mint a key by hand: ${PKG_BIN} subscription generate)"
