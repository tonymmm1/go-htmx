#!/usr/bin/env bash

# Download the CSS toolchain into .tools/: the Tailwind CSS standalone CLI (a
# single executable with the typography plugin built in) and daisyUI's
# standalone plugin bundle. No Node.js or npm required.
#
# Every file is verified against the SHA-256 checksums pinned below. To upgrade,
# change the version, then copy the new checksums from the release's
# sha256sums.txt (Tailwind) or the asset digests (`gh release view <tag> -R
# saadeghi/daisyui --json assets`).
#
# Usage: bash scripts/install-tools.sh   (a no-op when already installed)

set -euo pipefail

readonly TAILWIND_VERSION="v4.3.5"
readonly DAISYUI_VERSION="v5.7.50"

tailwind_sha256() {
    case "$1" in
        tailwindcss-linux-arm64) echo ffcd0d21dbd7c605dc12743ff0c0b47debc963c86b399674084c1c8f2c96ecad ;;
        tailwindcss-linux-arm64-musl) echo 564dad3028c029b0c69042c66a7a79873ed7289b73fcb7b37973fbf79cb9a32f ;;
        tailwindcss-linux-x64) echo e47695cee3f0d90623a91f3939b08bdee610eb5ad99340adff9d4145b284d6cc ;;
        tailwindcss-linux-x64-musl) echo bb050edd13f15006d872a9bb1772b908618a8df6806d30bf095cba26a09e640b ;;
        tailwindcss-macos-arm64) echo f3f4f5d1727e2b693b8f1c9a1492b5dd87008c86ea286bee1589cb458e6235fc ;;
        tailwindcss-macos-x64) echo a63702686a3ed048b3135d84a73be49ea9d47f880c9b20e54b3ef60934ad8cdf ;;
        tailwindcss-windows-x64.exe) echo 2803583268c53f4713e89943e211081fddb186c0a4a7d07b5c342c6ae853ec95 ;;
        *) return 1 ;;
    esac
}
readonly DAISYUI_SHA256="d69de07c7463bb1f41548e2688f88ee105cdadf9779fce5b3c47b3bfaa6be472"

readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly TOOLS_DIR="$SCRIPT_DIR/../.tools"
readonly STAMP="$TOOLS_DIR/.versions"
readonly WANT="tailwindcss $TAILWIND_VERSION, daisyui $DAISYUI_VERSION"

# Release asset name for this machine.
tailwind_asset() {
    local os arch
    case "$(uname -s)" in
        Linux) os=linux ;;
        Darwin) os=macos ;;
        MINGW* | MSYS* | CYGWIN*) echo "tailwindcss-windows-x64.exe"; return ;;
        *) echo "Unsupported OS: $(uname -s)" >&2; return 1 ;;
    esac
    case "$(uname -m)" in
        x86_64 | amd64) arch=x64 ;;
        aarch64 | arm64) arch=arm64 ;;
        *) echo "Unsupported CPU architecture: $(uname -m)" >&2; return 1 ;;
    esac
    local asset="tailwindcss-$os-$arch"
    # Alpine and other musl-based Linux distributions need the musl build.
    if [[ $os == linux ]] && { [[ -e /etc/alpine-release ]] || ldd --version 2>&1 | grep -qi musl; }; then
        asset="$asset-musl"
    fi
    echo "$asset"
}

sha256() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | cut -d' ' -f1
    else
        shasum -a 256 "$1" | cut -d' ' -f1
    fi
}

# download URL DEST SHA256
download() {
    local tmp="$2.download"
    echo "Downloading $1"
    curl -fsSL --retry 3 -o "$tmp" "$1"
    local got
    got="$(sha256 "$tmp")"
    if [[ "$got" != "$3" ]]; then
        rm -f -- "$tmp"
        echo "Checksum mismatch for $1" >&2
        echo "  want $3" >&2
        echo "  got  $got" >&2
        return 1
    fi
    mv -- "$tmp" "$2"
}

readonly ASSET="$(tailwind_asset)"
if [[ "$ASSET" == *.exe ]]; then
    readonly BINARY="$TOOLS_DIR/tailwindcss.exe"
else
    readonly BINARY="$TOOLS_DIR/tailwindcss"
fi

if [[ -x "$BINARY" && -f "$TOOLS_DIR/daisyui.mjs" && "$(cat "$STAMP" 2>/dev/null)" == "$WANT" ]]; then
    exit 0
fi

mkdir -p "$TOOLS_DIR"
download "https://github.com/tailwindlabs/tailwindcss/releases/download/$TAILWIND_VERSION/$ASSET" \
    "$BINARY" "$(tailwind_sha256 "$ASSET")"
chmod +x "$BINARY"
download "https://github.com/saadeghi/daisyui/releases/download/$DAISYUI_VERSION/daisyui.mjs" \
    "$TOOLS_DIR/daisyui.mjs" "$DAISYUI_SHA256"
echo "$WANT" > "$STAMP"
echo "Installed $WANT into .tools/"
