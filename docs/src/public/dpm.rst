.. Copyright (c) 2023 Digital Asset (Switzerland) GmbH and/or its affiliates. All rights reserved.
.. SPDX-License-Identifier: Apache-2.0

.. _dpm:

Digital Asset Package Manager (Dpm)
###################################


``dpm`` is a command-line tool that does a lot of useful things related to the
SDK. It is a **drop-in replacement** for the now deprecated
:subsiteref:`Daml Assistant<daml-assistant>`.

Pre-requisites
**************

Dpm currently runs on Windows, macOS and Linux.

For full functionality, you must have installed:

1) `vscode download <https://code.visualstudio.com/download>`_
2) JDK 17 or greater, installed and part of your `JAVA_HOME`.  If you do not already have a JDK installed, try OpenJDK or `Eclipse Adoptium <https://adoptium.net/>`_.


.. _dpm-install:

Install
*******

When installing Dpm, you can set the ``DPM_HOME`` environment variable to change the location where the SDK and any future updates are installed. The default is

- ``${HOME}/.dpm/`` on mac and linux
- ``%APPDATA%/.dpm/`` on windows


.. _dpm-installation:

Installation Instructions
=========================

To install the latest version:


.. _mac-linux-initial-install:

Mac/Linux Installation
----------------------

.. code:: shell

   > curl https://get.digitalasset.com/install/install.sh | sh


Windows Installation
--------------------

Download and run the `windows installer <https://get.digitalasset.com/install/latest-windows.html>`_, which will install the dpm sdk and set up the PATH variable for you.

.. _dpm-manual-installation:

Manual Installation Instructions
================================

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
    TARBALL_URL="https://artifactregistry.googleapis.com/download/v1/projects/da-images/locations/europe/repositories/public-generic/files/dpm-sdk:${VERSION}:${TARBALL}:download?alt=media"

    # make tmpdir
    TMPDIR="$(mktemp -d)"

    # download tarball
    curl -SLf "${TARBALL_URL}" --output "${TMPDIR}/${TARBALL}" --progress-bar "$@"

    # create directory to extract into
    extracted="${TMPDIR}/extracted"
    mkdir -p "${extracted}"

    # untar
    tar xzf "${TMPDIR}/${TARBALL}" -C "${extracted}" --strip-components 1

    # bootstrap dpm
    "${extracted}/bin/dpm" bootstrap "${extracted}"


.. _dpm-manual-managing-releases:

Managing and Upgrading SDK Versions
===================================

You can manage SDK versions manually by using ``dpm install``.

To install the SDK version specified in the daml.yaml, run:

.. code:: shell

  > dpm install package

To install a specific SDK version, for example version ``3.4.9``, run:

.. code:: shell

  > dpm install 3.4.9

To see the active SDK version

.. code:: shell

  > dpm version --active
  3.4.0-snapshot.20251002.1372.4055ddd

To list the installed SDK versions, including the currently active one (marked with `*`):

.. code:: shell

  > dpm version
    3.4.0-snapshot.20251003.1412.3fe167f
  * 3.4.0-snapshot.20251006.1451.85eca5a

To additionally list all the SDK versions that can be installed, as well as the installed versions:

.. code:: shell

  > dpm version --all

To get the list in a machine readable format:

.. code:: shell

  > dpm version --all -o json

.. code:: json

     [
        {
            "version": "3.4.0-rc2",
            "remote": true
        },
        {
            "version": "3.4.8",
            "installed": true,
            "remote": true
        },
        {
            "version": "3.4.9",
            "installed": true,
            "remote": true,
            "active": true
        }
    ]

.. _dpm_unstable_releases:

Unstable releases
=================
To install unstable SDKs you need to :ref:`configure dpm <dpm-configuration>` to look for them by setting the 
``registry`` configuration field or ``DPM_REGISTRY`` environment variable to `europe-docker.pkg.dev/da-images/public-unstable`, then you can use the same ``dpm install`` command:

.. code:: shell

  > dpm install <unstable SDK version>


.. _dpm-operate:

Operate
*******

- ``dpm build``:                    Build a Daml package or project

  This builds the Daml project according to the project config file ``daml.yaml`` (see `configuration files <dpm-configuration-files>`).

  In particular, it will use the dpm SDK (specified in the ``sdk-version`` field in ``daml.yaml``) to resolve dependencies and compile the Daml project.

  Given a ``daml.yaml`` and ``.daml`` source files, the ``dpm build`` command will generate a .dar for this package. See :externalref:`How to build Daml Archives <build_howto_build_dar_files>` for how to define a package and build it to a DAR.

- ``dpm test``:                     Test the current Daml project or the given files by running all test declarations.

  This runs all daml scripts defined within a package.

  Daml Scripts are top level values of type ``Script ()``, from the ``daml-script`` package. This package mimics a Canton Ledger Client for quick iterative testing,
  and direct support within :externalref:`Daml Studio <daml-studio>`. The command runs these scripts against a reference Ledger called the IDE Ledger, which implements the core functionality of the Canton Ledger without the complexity of multi-participant setups.
  
  It is most useful for verifying the fundamentals of your ledger model, before moving onto integration testing via the Ledger API directly, or the Daml Codegen.  ``dpm test`` also provides code coverage information for templates and choices used.

- ``dpm clean``:                    Clean a Daml package or project

  This removes any Daml artifact files created in your package during a daml build, including DARs.

- ``dpm codegen-alpha-java``:       codegen (alpha) for java
- ``dpm codegen-alpha-scala``:      codegen (alpha) for scala
- ``dpm codegen-alpha-typescript``: codegen (alpha) for typescript
- ``dpm codegen-java``:             Daml to Java compiler
- ``dpm codegen-js``:               Daml to JavaScript compiler
- ``dpm daml-shell``:               daml-shell client for PQS
- ``dpm damlc``:                    Compiler and IDE backend for the Daml programming language
- ``dpm docs``:                     Generate documentation for a daml package from its documentation comments
- ``dpm inspect-dar``:              Inspect a DAR archive
- ``dpm new``:                      Create a new Daml package
- ``dpm pqs``:                      participant query store
- ``dpm sandbox``:                  Run full Canton installation in a single process
- ``dpm script``:                   Daml Script Binary
- ``dpm studio``:                   Launch Daml Studio

..  You can disable the HTTP JSON API by passing ``--json-api-port none`` to ``daml start``.
  To specify additional options for sandbox/navigator/the HTTP JSON API you can use
  ``--sandbox-option=opt``, ``--navigator-option=opt`` and ``--json-api-option=opt``.

- ``dpm validate-dar``              Validate a DAR archive
- ``dpm install``:                  Install new SDK versions manually

  Note that you need to update your `project config file <#configuration-files>` to use the new version.


.. _dpm-daml-assistant-to-dpm-migration:

``daml`` assistant to ``dpm`` command migration
***********************************************

+-----------------------------------------+--------------------------------+----------------------------------------------------+
| daml command                            | dpm command                    | purpose                                            |
+=========================================+================================+====================================================+
| daml new                                | dpm new                        | Create a new Daml project                          |
+-----------------------------------------+--------------------------------+----------------------------------------------------+
| daml build                              | dpm build                      | Compile the Daml project                           |
+-----------------------------------------+--------------------------------+----------------------------------------------------+
| daml test                               | dpm test                       | Run tests for the Daml project                     |
+-----------------------------------------+--------------------------------+----------------------------------------------------+
| daml install                            | dpm install                    | Install Daml SDK components                        |
+-----------------------------------------+--------------------------------+----------------------------------------------------+
| daml codegen java                       | dpm codegen-java               | Java code generation                               |
+-----------------------------------------+--------------------------------+----------------------------------------------------+
| daml codegen js                         | dpm codegen-js                 | TypeScript code generation                         |
+-----------------------------------------+--------------------------------+----------------------------------------------------+
| daml damlc                              | dpm damlc                      | Invoke the daml compiler                           |
+-----------------------------------------+--------------------------------+----------------------------------------------------+
| daml studio                             | dpm studio                     | Open project in Visual Studio                      |
+-----------------------------------------+--------------------------------+----------------------------------------------------+
| daml sandbox                            | dpm sandbox                    | Launch a Daml Sandbox                              |
+-----------------------------------------+--------------------------------+----------------------------------------------------+
| daml ledger allocate-parties            | Use Declarative API            | Allocate parties on a ledger                       |
|                                         | – OR –                         |                                                    |
|                                         | JSON / gRPC API                |                                                    |
+-----------------------------------------+--------------------------------+----------------------------------------------------+
| daml ledger list-parties                | JSON / gRPC API                | list parties on a ledger                           |
+-----------------------------------------+--------------------------------+----------------------------------------------------+
| daml ledger upload-dar                  | Use Declarative API            | Upload (and vet) dars on a ledger                  |
|                                         | – OR –                         |                                                    |
|                                         | JSON / gRPC API                |                                                    |
+-----------------------------------------+--------------------------------+----------------------------------------------------+
| daml ledger fetch-dar                   | gRPC API                       | Fetch a Dar from a ledger.                         |
+-----------------------------------------+--------------------------------+----------------------------------------------------+
| daml start                              | dpm sandbox                    | Start a local Daml Ledger                          |
|                                         | dpm build                      |                                                    |
|                                         | Use Declarative API            |                                                    |
|                                         | – OR –                         |                                                    |
|                                         | JSON / gRPC to upload/allocate |                                                    |
+-----------------------------------------+--------------------------------+----------------------------------------------------+
| daml packages                           | JSON / gRPC API                | Package a Daml project                             |
+-----------------------------------------+--------------------------------+----------------------------------------------------+


.. _dpm-help:

Command Help
************

To see information about any command, run it with ``--help``.
