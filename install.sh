#!/bin/sh
set -eu

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) echo 'Unsupported OS' >&2; exit 1 ;;
esac
case "$(uname -m)" in
  arm64|aarch64) arch=arm64 ;;
  x86_64|amd64) arch=amd64 ;;
  *) echo 'Unsupported CPU architecture' >&2; exit 1 ;;
esac

install_dir=
for dir in "$HOME/.local/bin" "$HOME/bin" /opt/homebrew/bin /usr/local/bin; do
  if [ -d "$dir" ] && [ -w "$dir" ]; then
    case ":$PATH:" in
      *":$dir:"*) install_dir=$dir; break ;;
    esac
  fi
done
if [ -z "$install_dir" ]; then
  install_dir="$HOME/.local/bin"
  mkdir -p "$install_dir"
fi

url="https://github.com/riskibarqy/diffscribe/releases/latest/download/diffscribe-$os-$arch"
tmp=$(mktemp "$install_dir/.cmg.XXXXXX")
trap 'rm -f "$tmp"' EXIT
curl -fL "$url" -o "$tmp"
chmod 755 "$tmp"
mv "$tmp" "$install_dir/cmg"
echo "Installed $install_dir/cmg"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) echo "Add this to your shell startup file: export PATH=\"$install_dir:\$PATH\"" ;;
esac
