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

readonly TAILWIND_VERSION="v4.3.3"
readonly DAISYUI_VERSION="v5.7.22"

tailwind_sha256() {
    case "$1" in
        tailwindcss-linux-arm64) echo 55fd0b241214eff3de1e8ee4f22796662f2d2e7a49bcfca7477cfd0bac398195 ;;
        tailwindcss-linux-arm64-musl) echo 71ea4be79c9de9827545682df3e040053fb535d37c71ed2cfdedf9385a0868e0 ;;
        tailwindcss-linux-x64) echo dc61b3ac6b8c9ca874c0cc4c57b2409791a64c5540404ca5f5367360babc313a ;;
        tailwindcss-linux-x64-musl) echo a04d34ceacc8f52cbe8920ad846cdeb61d3d0021dba32db0d1f77c9d9fad7a6c ;;
        tailwindcss-macos-arm64) echo cdf646702987a743464dff4d9c60fd4480d1c1e73dd819a9a67f1078815dce9d ;;
        tailwindcss-macos-x64) echo 7922e0953f2110c05976e3bf58f14e643d90427575e766b7d433f5f80cbee7e1 ;;
        tailwindcss-windows-x64.exe) echo e0e260ce048014e9268f6237ff18f8ccf02cef521cbd0ae04e82c2cdf7aa3955 ;;
        *) return 1 ;;
    esac
}
readonly DAISYUI_SHA256="abd307ebaa5913d26518a55d1946da89ebdd58dfcdf880a9a862c938965d3784"

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
