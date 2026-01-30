.. Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
.. SPDX-License-Identifier: Apache-2.0

.. _dpm-manual-installation:

Manual Installation Instructions
================================

Mac/Linux
---------

If you cannot / wish not to use the shell script to install for Linux or OSX, you can alternatively install dpm manually by running this set of commands in your terminal:

The latest stable release version can be found by 

.. code:: shell
   VERSION="$(curl -sS "https://get.digitalasset.com/install/latest")"

And you can then use this to retrieve the tarball of the full installation, extract, and install, as outlined in the full instructions below.

.. code:: shell

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

Windows
-------

Download and unpack the latest dpm sdk version's `archive (.zip) <https://get.digitalasset.com/install/latest-windows-archive.html>`_, then:

.. code:: powershell

    # Extract the downloaded zip ($ZIP_PATH) to temp directory ($EXTRACTED)
    # Avoid using the system's temp directory as the user may not have rights to it
    New-Item -ItemType Directory -Path $EXTRACTED | Out-Null
    Expand-Archive -Path $ZIP_PATH -DestinationPath $EXTRACTED
    
    # Optionally, override the TMP and DPM_HOME environment variable to point to directories other than the default,
    # as the user might not have rights to the default directories.
    # (You might also want to persist these variables as DPM uses them on every invocation)
    $env:TMP = "<user-owned temporary directory>"
    $env:DPM_HOME = "<user-owned directory>"

    & "$EXTRACTED\windows-amd64\bin\dpm.exe" bootstrap $EXTRACTED\windows-amd64


Unstable Versions
-----------------

Preview / unstable versions are also available for experimentation, though it is always recommended to use the stable versions listed above.


Unstable Mac / Linux
--------------------

.. code:: shell
   UNSTABLE_VERSION="$(curl -sS "https://get.digitalasset.com/unstable/install/latest")"

   UNSTABLE_TARBALL_URL="https://get.digitalasset.com/unstable/install/dpm-sdk/${UNSTABLE_VERSION}"


Unstable Windows
--------------------

Download and unpack the latest unstable dpm sdk version's `archive (.zip) <https://get.digitalasset.com/unstable/install/latest-windows-archive.html>`_, then:

