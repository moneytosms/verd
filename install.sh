#!/bin/sh
# Install verd from the latest GitHub release.
#   curl -fsSL https://raw.githubusercontent.com/moneytosms/verd/main/install.sh | sh
# VERD_VERSION=v0.1.0 pins a version; VERD_INSTALL_DIR sets the target (default ~/.local/bin).
set -eu

repo=moneytosms/verd
dir=${VERD_INSTALL_DIR:-$HOME/.local/bin}

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case $os in linux|darwin) ;; *) echo "verd: unsupported OS $os" >&2; exit 1 ;; esac
case $(uname -m) in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "verd: unsupported CPU $(uname -m)" >&2; exit 1 ;;
esac

tag=${VERD_VERSION:-}
if [ -z "$tag" ]; then
  tag=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")
  tag=${tag##*/}
fi
case $tag in v[0-9]*) ;; *) echo "verd: no release found (got '$tag')" >&2; exit 1 ;; esac

name=verd_${tag#v}_${os}_${arch}.tar.gz
base=https://github.com/$repo/releases/download/$tag
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

curl -fsSL -o "$tmp/$name" "$base/$name"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt"
want=$(awk -v n="$name" '$2 == n {print $1}' "$tmp/checksums.txt")
if command -v sha256sum >/dev/null; then got=$(sha256sum "$tmp/$name" | awk '{print $1}')
else got=$(shasum -a 256 "$tmp/$name" | awk '{print $1}'); fi
[ -n "$want" ] && [ "$want" = "$got" ] || { echo "verd: checksum mismatch for $name" >&2; exit 1; }

tar -xzf "$tmp/$name" -C "$tmp" verd
mkdir -p "$dir"
install -m 755 "$tmp/verd" "$dir/verd"
echo "installed verd $tag to $dir/verd"
case ":$PATH:" in *":$dir:"*) ;; *) echo "add $dir to your PATH" ;; esac
