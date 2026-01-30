#! /usr/bin/env bash
set -xeuo pipefail

#get latest version number
VERSION="$(curl -sS "https://get.digitalasset.com/install/latest")"

# set your architecture to either amd64 | arm64
ARCH="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"

# set your OS to either darwin or linux
OS="$(uname | tr '[:upper:]' '[:lower:]')"

#pull down appropriate tarball for your OS and architecture
readonly TARBALL="dpm-${VERSION}-${OS}-${ARCH}.tar.gz"

# determine location of tarball to download
TARBALL_URL="https://get.digitalasset.com/install/dpm-sdk/${TARBALL}"

# make tmpdir
TMPDIR="$(mktemp -d)"

# download tarball
curl -SLf "${TARBALL_URL}" --output "${TMPDIR}/${TARBALL}" --progress-bar "$@"

# create directory to extract into
extracted="${TMPDIR}/extracted"
mkdir -p "${extracted}"

# untar to extracted directory
tar xzf "${TMPDIR}/${TARBALL}" -C "${extracted}" --strip-components 1

# bootstrap dpm
"${extracted}/bin/dpm" bootstrap "${extracted}"

# cleanup tmpdir
rm -rf "${TMPDIR}"
