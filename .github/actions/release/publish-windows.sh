#!/usr/bin/env bash
set -euo pipefail

VERSION="$(cat dist/metadata.json | jq -r .version)"

installer="dpm-${VERSION}-windows-amd64.exe"
mv "dist/dpm-installer.exe" "dist/${installer}"
zip -j "dist/${installer}.zip" "dist/${installer}" "LICENSE"

gsutil cp -n "dist/${installer}" "gs://da-images-public/install/windows/dpm/${installer}"
gsutil cp -n "dist/${installer}.zip" "gs://da-images-public/install/windows/dpm/${installer}.zip"
