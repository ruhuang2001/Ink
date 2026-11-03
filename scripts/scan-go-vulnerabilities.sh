#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../server"
go version
go env GOVERSION GOTOOLCHAIN
packages=$(go list ./cmd/...)
test -n "$packages"
scan_dir="$(mktemp -d)"
trap 'rm -rf "$scan_dir"' EXIT
tool_dir="${HOME}/.cache/ink-tools/govulncheck-v1.7.0"
mkdir -p "$tool_dir"
if [ ! -x "$tool_dir/govulncheck" ]; then
  GOBIN="$tool_dir" go install golang.org/x/vuln/cmd/govulncheck@v1.7.0
fi
for package in $packages; do
  binary="$scan_dir/${package##*/}"
  go build -o "$binary" "$package"
  go version -m "$binary"
  "$tool_dir/govulncheck" -mode=binary "$binary"
done
