#!/bin/sh
set -eu

repo="Timmyy3000/git-forest"
binary="forest"
version="${FOREST_VERSION:-latest}"

fail() {
  echo "forest install: $*" >&2
  exit 1
}

have() {
  command -v "$1" >/dev/null 2>&1
}

detect_os() {
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    linux) echo "linux" ;;
    darwin) echo "darwin" ;;
    *) fail "unsupported OS: $os" ;;
  esac
}

detect_arch() {
  arch="$(uname -m)"
  case "$arch" in
    x86_64 | amd64) echo "amd64" ;;
    arm64 | aarch64) echo "arm64" ;;
    *) fail "unsupported architecture: $arch" ;;
  esac
}

can_write_dir() {
  dir="$1"
  mkdir -p "$dir" 2>/dev/null || return 1
  test_file="$dir/.forest-install-test"
  ( : > "$test_file" ) 2>/dev/null || return 1
  rm -f "$test_file"
}

select_install_dir() {
  if [ "${FOREST_INSTALL_DIR:-}" ]; then
    can_write_dir "$FOREST_INSTALL_DIR" || fail "cannot write to FOREST_INSTALL_DIR=$FOREST_INSTALL_DIR"
    echo "$FOREST_INSTALL_DIR"
    return
  fi

  if can_write_dir "/usr/local/bin"; then
    echo "/usr/local/bin"
    return
  fi

  [ -n "${HOME:-}" ] || fail "HOME is not set and /usr/local/bin is not writable"
  home_bin="${HOME}/.local/bin"
  can_write_dir "$home_bin" || fail "cannot write to /usr/local/bin or $home_bin"
  echo "$home_bin"
}

download() {
  url="$1"
  dest="$2"
  if have curl; then
    curl -fsSL "$url" -o "$dest"
  elif have wget; then
    wget -qO "$dest" "$url"
  else
    fail "curl or wget is required"
  fi
}

verify_checksum() {
  archive="$1"
  checksum_line="$(awk -v archive="$archive" '$2 == archive { print; found = 1 } END { exit found ? 0 : 1 }' checksums.txt || true)"
  [ -n "$checksum_line" ] || fail "checksum entry not found for $archive"
  checksum_file="checksum.txt"
  printf '%s\n' "$checksum_line" > "$checksum_file"

  if have sha256sum; then
    sha256sum -c "$checksum_file" >/dev/null
  elif have shasum; then
    shasum -a 256 -c "$checksum_file" >/dev/null
  else
    echo "forest install: warning: sha256sum or shasum not found; skipping checksum verification" >&2
  fi
}

os="$(detect_os)"
arch="$(detect_arch)"
archive="forest_${os}_${arch}.tar.gz"

if [ "$version" = "latest" ]; then
  base_url="https://github.com/${repo}/releases/latest/download"
else
  base_url="https://github.com/${repo}/releases/download/${version}"
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT INT TERM

cd "$tmp_dir"
download "${base_url}/${archive}" "$archive"
download "${base_url}/checksums.txt" checksums.txt
verify_checksum "$archive"

tar -xzf "$archive" "$binary"

install_dir="$(select_install_dir)"
install_path="${install_dir}/${binary}"
cp "$binary" "$install_path"
chmod 755 "$install_path"

echo "forest installed to ${install_path}"

case ":$PATH:" in
  *":${install_dir}:"*) ;;
  *) echo "forest install: warning: ${install_dir} is not on PATH" >&2 ;;
esac

"$install_path" version
