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

.. literalinclude:: manual-install.sh
   :caption: Manual installation script for Mac/Linux
   :language: shell
   :linenos:


Windows
-------

Download and unpack the latest dpm sdk version's `archive (.zip) <https://get.digitalasset.com/install/latest-windows-archive.html>`_, then:

.. literalinclude:: manual-install-windows.ps1
   :caption: Manual installation script for Windows
   :language: powershell
   :linenos:




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

