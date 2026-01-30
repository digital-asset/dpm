.. Copyright (c) 2026 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
.. SPDX-License-Identifier: Apache-2.0

.. _dpm-manual-installation:

Manual Installation Instructions
================================

.. _dpm-manual-installation-mac-linux:

Mac/Linux Installation
----------------------

If you cannot / wish not to use the shell script to install for Linux or OSX, you can alternatively install dpm manually by running this set of commands in your terminal:

The latest stable release version can be found by hitting the following URL:

.. code:: shell

   VERSION="$(curl -sS "https://get.digitalasset.com/install/latest")"

And you can then use this to retrieve the tarball of the full installation, extract, and install, as outlined in the full instructions below.

.. literalinclude:: manual-install.sh
   :caption: Manual installation for Mac/Linux
   :language: shell
   :lines: 4-


.. _dpm-manual-installation-windows:

Windows Installation
--------------------

Download and unpack the latest windows dpm sdk `archive (.zip) <https://get.digitalasset.com/install/latest-windows-archive.html>`_, then:

.. literalinclude:: manual-install-windows.ps1
   :caption: Manual installation script for Windows
   :language: powershell
   :linenos:




Unstable Version Manual Installation
====================================

Preview / unstable versions are also available for experimentation, though it is always recommended to use the stable versions listed above instead.


Unstable Mac / Linux
--------------------

Follow the :ref:`dpm-manual-installation-mac-linux` instructions to install an unstable version, but note the differences in VERSION and TARBALL_URL.

The latest unstable release version can be found by hitting the following URL:

.. code:: shell

   VERSION="$(curl -sS "https://get.digitalasset.com/unstable/install/latest")"

The unstable tarball can be retrieved from the following URL:

.. code:: shell

   # determine location of tarball to download
   TARBALL_URL="https://get.digitalasset.com/unstable/install/dpm-sdk/${TARBALL}"


Unstable Windows
----------------

Download and unpack the latest unstable windows dpm sdk `archive (.zip) <https://get.digitalasset.com/unstable/install/latest-windows-archive.html>`_, then follow :ref:`dpm-manual-installation-windows` instructions.

