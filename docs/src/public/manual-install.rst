.. Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
.. SPDX-License-Identifier: Apache-2.0

.. _dpm-manual-installation:

Manual Installation Instructions
================================

Mac/Linux
---------

If you cannot / wish not to use the shell script to install for Linux or OSX, you can alternatively install dpm manually by running this set of commands in your terminal:

.. code:: shell

    #get latest version number
    readonly VERSION="$(curl -sS "https://get.digitalasset.com/install/latest")"

    # set your architecture to either amd64 | arm64
    readonly ARCH="$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')"

    # set your OS to either darwin or linux
    readonly OS="$(uname | tr '[:upper:]' '[:lower:]')"

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

Windows
-------

Download and unpack the latest dpm sdk version's `archive (.zip) <https://get.digitalasset.com/install/latest-windows-archive.html>`_, then:

.. code:: powershell

    # Extract the downloaded zip ($ZIP_PATH) to temp directory
    $EXTRACTED = Join-Path "$env:TEMP" "extracted"
    New-Item -ItemType Directory -Path $EXTRACTED | Out-Null
    Expand-Archive -Path $ZIP_PATH -DestinationPath $EXTRACTED
    
    & "$EXTRACTED\windows-amd64\bin\dpm.exe" bootstrap $EXTRACTED\windows-amd64
