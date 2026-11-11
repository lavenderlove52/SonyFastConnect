#!/usr/bin/env bash
set -euo pipefail

GO_VERSION="1.26.2"
DOWNLOAD_BASE="https://go.dev/dl"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

case "$(uname -s)" in
    Darwin) platform="darwin" ;;
    *)
        echo "This script supports macOS only. Use install-go.cmd on Windows." >&2
        exit 1
        ;;
esac

case "$(uname -m)" in
    arm64) architecture="arm64" ;;
    x86_64) architecture="amd64" ;;
    *)
        echo "Unsupported macOS architecture: $(uname -m)" >&2
        exit 1
        ;;
esac

case "$architecture" in
    arm64) expected_sha256="32af1522bf3e3ff3975864780a429cc0b41d190ec7bf90faa661d6d64566e7af" ;;
    amd64) expected_sha256="bc3f1500d9968c36d705442d90ba91addf9271665033748b82532682e90a7966" ;;
esac

go_is_compatible() {
    command -v go >/dev/null 2>&1 || return 1
    local version major minor
    version="$(go env GOVERSION 2>/dev/null)" || return 1
    if [[ ! "$version" =~ ^go([0-9]+)\.([0-9]+) ]]; then
        return 1
    fi
    major="${BASH_REMATCH[1]}"
    minor="${BASH_REMATCH[2]}"
    (( major > 1 || (major == 1 && minor >= 26) ))
}

if go_is_compatible; then
    echo "Compatible Go is already installed: $(go version)"
else
    install_root="${HOME}/.local/go/${GO_VERSION}"
    archive_name="go${GO_VERSION}.${platform}-${architecture}.tar.gz"
    download_url="${DOWNLOAD_BASE}/${archive_name}"
    work_root="$(mktemp -d "${TMPDIR:-/tmp}/sonyfastconnect-go.XXXXXX")"
    trap 'rm -rf "${work_root}"' EXIT
    archive_path="${work_root}/${archive_name}"

    if [[ -d "$install_root" ]]; then
        echo "Install directory already exists but is not a compatible Go installation: $install_root" >&2
        exit 1
    fi

    echo "Downloading official Go ${GO_VERSION} for macOS ${architecture}..."
    curl --fail --location --retry 3 --output "$archive_path" "$download_url"
    actual_sha256="$(shasum -a 256 "$archive_path" | awk '{print $1}')"
    if [[ "$actual_sha256" != "$expected_sha256" ]]; then
        echo "Go archive checksum mismatch." >&2
        echo "Expected: $expected_sha256" >&2
        echo "Actual:   $actual_sha256" >&2
        exit 1
    fi

    mkdir -p "$install_root"
    tar -xzf "$archive_path" -C "$install_root" --strip-components 1
    if [[ ! -x "$install_root/bin/go" ]]; then
        echo "Installed archive is missing $install_root/bin/go." >&2
        exit 1
    fi

    export PATH="$install_root/bin:$PATH"
    case "${SHELL:-}" in
        */zsh) profile="${HOME}/.zshrc" ;;
        */bash) profile="${HOME}/.bash_profile" ;;
        *) profile="${HOME}/.profile" ;;
    esac
    marker="# SonyFastConnect Go ${GO_VERSION}"
    if ! grep -Fq "$marker" "$profile" 2>/dev/null; then
        {
            echo ""
            echo "$marker"
            echo "export PATH=\"$install_root/bin:\$PATH\""
        } >> "$profile"
    fi
fi

if ! go_is_compatible; then
    echo "Go is still not available after installation. Open a new terminal and rerun this script." >&2
    exit 1
fi

echo "Using: $(go version)"
cd "$SCRIPT_DIR"
echo "Downloading pinned Go module dependencies..."
go mod download
echo "Running probe tests..."
go test ./...
echo "Building a7m5-probe..."
GOOS=darwin GOARCH="$architecture" go build -trimpath -o a7m5-probe .
chmod +x a7m5-probe

echo "Ready: ${SCRIPT_DIR}/a7m5-probe"
