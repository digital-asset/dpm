#!/usr/bin/env bash
set -euo pipefail

script_path="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo $script_path
GOOS=windows GOARCH=amd64 go build \
  -o "${script_path}/windows/dpm.exe" \
  "${script_path}/mockwindows/main.go"
