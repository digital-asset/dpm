.. Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
.. SPDX-License-Identifier: Apache-2.0

.. _dpm-manual-installation:

Manual Installation Instructions
================================

Mac/Linux
---------

If you cannot / wish not to use the shell script to install for Linux or OSX, you can alternatively install dpm manually by running this set of commands in your terminal:

The latest stable release version can be found by hitting the following URL:

.. code:: shell
   VERSION="$(curl -sS "https://get.digitalasset.com/install/latest")"

And you can then use this to retrieve the tarball of the full installation, extract, and install, as outlined in the full instructions below.

.. literalinclude:: manual-install.sh
   :caption: Manual installation script for Mac/Linux
   :language: shell
   :lineno-start: 4


.. _dpm-manual-installation-windows:

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

You can follow the same :ref:dpm-manual-installation above to install an unstable version, but note the changes in the VERSION and TARBALL_URL variables below:

.. code:: shell
   VERSION="$(curl -sS "https://get.digitalasset.com/unstable/install/latest")"

   TARBALL_URL="https://get.digitalasset.com/unstable/install/dpm-sdk/${UNSTABLE_VERSION}"


Unstable Windows
--------------------

Download and unpack the latest unstable dpm sdk version's `archive (.zip) <https://get.digitalasset.com/unstable/install/latest-windows-archive.html>`_, then follow :ref:dpm-manual-installation-windows above.

