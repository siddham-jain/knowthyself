#!/bin/sh
# reflect installer — https://github.com/siddham/reflect
#
# Usage:   curl -fsSL https://<domain>/install.sh | sh
# Inspect: curl -fsSL https://<domain>/install.sh          (no pipe — prints this script)
#
# Downloads the right prebuilt binary for your OS/arch from GitHub Releases, verifies
# its SHA-256 against the signed checksums, installs it, and offers to run it.
set -eu

REPO="siddham/reflect"
BIN="reflect"

# --- colors (amber wordmark; disabled when piped or NO_COLOR) --------------------
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
	A='\033[1;38;5;208m' # amber
	D='\033[38;5;240m'   # dim grey (the reflection)
	B='\033[1m'
	Z='\033[0m'
else
	A='' D='' B='' Z=''
fi

banner() {
	printf '\n'
	for row in \
		"████  █████ █████ █     █████ █████ █████" \
		"█   █ █     █     █     █     █       █  " \
		"████  ███   ███   █     ███   █       █  " \
		"█  █  █     █     █     █     █       █  " \
		"█   █ █████ █     █████ █████ █████   █  "; do
		printf '   %b%s%b\n' "$A" "$row" "$Z"
	done
	printf '   %b%s%b\n' "$D" "╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌" "$Z"
	# reflection: rows reversed, blocks softened
	for row in \
		"▒   ▒ ▒▒▒▒▒ ▒     ▒▒▒▒▒ ▒▒▒▒▒ ▒▒▒▒▒   ▒  " \
		"▒  ▒  ▒     ▒     ▒     ▒     ▒       ▒  " \
		"▒▒▒▒  ▒▒▒   ▒▒▒   ▒     ▒▒▒   ▒       ▒  " \
		"▒  ▒  ▒     ▒     ▒     ▒     ▒       ▒  " \
		"▒▒▒▒  ▒▒▒▒▒ ▒▒▒▒▒ ▒     ▒▒▒▒▒ ▒▒▒▒▒ ▒▒▒▒▒"; do
		printf '   %b%s%b\n' "$D" "$row" "$Z"
	done
	printf '   %bknow how you build.%b\n\n' "$D" "$Z"
}

fail() { printf 'reflect: %s\n' "$1" >&2; exit 1; }

banner

# --- detect platform -------------------------------------------------------------
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) fail "unsupported architecture: $arch" ;;
esac
case "$os" in
	darwin | linux) ;;
	*) fail "unsupported OS: $os — on Windows use install.ps1" ;;
esac

# --- resolve version -------------------------------------------------------------
VERSION="${REFLECT_VERSION:-}"
if [ -z "$VERSION" ]; then
	VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null |
		grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name" *: *"([^"]+)".*/\1/')
fi
[ -n "$VERSION" ] || fail "could not find a published release yet — check https://github.com/$REPO/releases"
num="${VERSION#v}"

asset="${BIN}_${num}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$VERSION"
printf '  downloading %b%s%b (%s)\n' "$B" "$asset" "$Z" "$VERSION"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fSL# "$base/$asset" -o "$tmp/$asset" || fail "download failed"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" || fail "could not fetch checksums"

# --- verify checksum -------------------------------------------------------------
if command -v sha256sum >/dev/null 2>&1; then SHA="sha256sum"; else SHA="shasum -a 256"; fi
grep " ${asset}\$" "$tmp/checksums.txt" >"$tmp/want.txt" || fail "no checksum entry for $asset"
( cd "$tmp" && $SHA -c want.txt >/dev/null 2>&1 ) || fail "checksum verification FAILED — aborting"
printf '  %bchecksum ok%b\n' "$A" "$Z"

# --- install ---------------------------------------------------------------------
tar -xzf "$tmp/$asset" -C "$tmp"
if [ -w /usr/local/bin ]; then DEST=/usr/local/bin; else DEST="$HOME/.local/bin"; fi
mkdir -p "$DEST"
install -m 0755 "$tmp/$BIN" "$DEST/$BIN"
printf '  installed to %b%s/%s%b\n' "$B" "$DEST" "$BIN" "$Z"

case ":$PATH:" in
	*":$DEST:"*) ;;
	*) printf '  %badd %s to your PATH:%b  export PATH="%s:$PATH"\n' "$D" "$DEST" "$Z" "$DEST" ;;
esac

# --- offer to run it now ---------------------------------------------------------
printf '\n  %bMeet yourself now?%b [Y/n] ' "$B" "$Z"
ans=""
[ -r /dev/tty ] && read -r ans </dev/tty || true
case "$ans" in
	n | N | no | NO)
		printf '\n  whenever you are ready:  %breflect%b\n\n' "$A" "$Z"
		;;
	*)
		printf '\n'
		exec "$DEST/$BIN"
		;;
esac
