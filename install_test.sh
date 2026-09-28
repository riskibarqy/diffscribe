#!/bin/sh
set -eu

test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT
mkdir -p "$test_dir/tools" "$test_dir/home"
printf 'binary\n' > "$test_dir/expected"
cat > "$test_dir/tools/uname" <<'EOF'
#!/bin/sh
case "$1" in
  -s) echo Linux ;;
  -m) echo aarch64 ;;
esac
EOF
cat > "$test_dir/tools/curl" <<'EOF'
#!/bin/sh
case "$2" in
  */diffscribe-linux-arm64) ;;
  *) exit 1 ;;
esac
while [ "$1" != '-o' ]; do shift; done
printf 'binary\n' > "$2"
EOF
chmod +x "$test_dir/tools/uname" "$test_dir/tools/curl"
HOME="$test_dir/home" PATH="$test_dir/tools:/usr/bin:/bin" sh install.sh > "$test_dir/output"
cmp "$test_dir/expected" "$test_dir/home/.local/bin/cmg"
grep -q 'Add this to your shell startup file' "$test_dir/output"
