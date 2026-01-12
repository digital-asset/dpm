.. _dpm-configuration:

Configuration
=============

``dpm`` has both global, and project-specific configurations

Global Configuration
********************

Configuration options
---------------------

``dpm`` can be configured via (simultaneously): 

- config file: ``${DPM_HOME}/dpm-config.yaml`` 

=================== ===================================================================================================================================
config              purpose
=================== ===================================================================================================================================
registry            override the default location where dpm pulls SDKs and SDK components. The values are ``europe-docker.pkg.dev/da-images/public`` for stable releases (default), and ``europe-docker.pkg.dev/da-images/public-unstable`` for unstable releases)
registry-auth-path  override the default auth (file) used for the registry
insecure            allow ``dpm`` to pull SDKs and SDK components from insecure (``http``) registries
=================== ===================================================================================================================================

- environment variables: which take precedence over similar fields in ``dpm-config.yaml``

======================= ====================================================================================================================================
config                  purpose
======================= ====================================================================================================================================
DPM_REGISTRY            override the default location where dpm pulls SDKs and SDK components. The values are ``europe-docker.pkg.dev/da-images/public`` for stable releases (default), and ``europe-docker.pkg.dev/da-images/public-unstable`` for unstable releases)
DPM_REGISTRY_AUTH       override the default auth (file) used for the registry
DPM_INSECURE_REGISTRY   allow ``dpm`` to pull SDKs and SDK components from insecure (``http``) registries
DPM_LOG_LEVEL           for controlling log level of commands like ``dpm install`` and ``dpm version``. (values: ``debug``, ``info``, ``error``, ``warn``)
DAML_PACKAGE            allows running ``dpm`` commands in a package context without having to be in its directory (e.g. ``DAML_PACKAGE=/path/to/package``)
DPM_SDK_VERSION         allows overriding the SDK version being used. It's a global override that overrides the sdk version specified in any and all daml.yaml(s). It also overrides the SDK version used outside package or multi-package context. It doesn't affect the behavior of the `install` command(s)
======================= ====================================================================================================================================

Project Configuration
*********************

.. _dpm-configuration-files:

``dpm`` also uses ``daml.yaml`` and ``multi-package.yaml`` for single- and multi-package project configuration.


Multi-package Configuration file (multi-package.yaml)
-----------------------------------------------------
The ``multi-package.yaml`` file is used to inform Daml Build and the IDE of projects containing multiple
connected Daml packages.

An example is given below:

.. code-block:: yaml

  packages:
    - ./path/to/package/a
    - ./path/to/package/b

``packages``: an optional list of directories containing Daml packages, and by extension, ``daml.yaml`` config files. These allow Daml Multi-Build to
find the source code for dependency DARs and build them in topological order.

The multi-package also includes a ``dars`` field, for providing additional information to Daml Studio.
See :externalref:`Daml Studio Jump to definition <daml-studio-jump-to-def>` for more details.

Environment Variable Interpolation
----------------------------------

.. _dpm-environment-variable-interpolation:

Both the ``daml.yaml`` and ``multi-package.yaml`` config files support environment variable interpolation on all string fields.
Interpolation takes the form of ``${MY_ENVIRONMENT_VARIABLE}``, which is replaced with the content of ``MY_ENVIRONMENT_VARIABLE`` from the
calling shell. These can be escaped and placed within strings according to the environment variable interpolation semantics.

This allows you to extract common data, such as the sdk-version, package-name, or package-version outside of a package's ``daml.yaml``. For example,
you can use an ``.envrc`` file or have these values provided by a build system. This feature can also be used for specifying dependency DARs, enabling you to either store
your DARs in a common folder and pass its directory as a variable, shortening the paths in your ``daml.yaml``, or pass each dependency as a
separate variable through an external build system, which may store them in a temporary cache.

The following example showcases this:

.. code-block:: yaml

  sdk-version: ${SDK_VERSION}
  name: ${PROJECT_NAME}_test
  source: daml
  version: ${PROJECT_VERSION}
  dependencies:
    // Using a common directory
    ${DEPENDENCY_DIRECTORY}/my-dependency-1.0.0.dar
    ${DEPENDENCY_DIRECTORY}/my-other-dependency-1.0.0.dar
    // Passed directly by a build system
    ${DAML_FINANCE_DAR}
    ${MY_DEPENDENCY_DAR}

Escape syntax uses the ``\`` prefix: ``\${NOT_INTERPOLATED}``.

.. _dpm-override-components:

(Advanced) Overriding SDK Components
-------------------------------------------------------

``dpm`` supports overriding components for an installed SDK for a single package and/or a multi-package project.

in ``daml.yaml``
~~~~~~~~~~~~~~~~

.. code:: yaml

   sdk-version: 3.4.0-snapshot.20251013.1566.e75a9cf
   override-components:
     damlc:
       version: 3.4.0-snapshot.20251007.14274.0.ve2024cd6

in ``multi-package.yaml``
~~~~~~~~~~~~~~~~~~~~~~~~~

You can use the same ``override-components`` yaml object in a ``multi-package.yaml`` too. This applies the specified overrides to all packages.

.. code:: yaml

   packages:
     - ./daml-pkg-1
     - ./daml-pkg-2

   override-components:
     damlc:
       version: 3.4.0-snapshot.20251007.14274.0.ve2024cd6

When both ``multi-package.yaml`` and one of its packages' ``daml.yaml`` simultaneously override components, ``dpm`` overlays ``daml.yaml``'s components on top of ``multi-package.yaml``'s:

- Starting with the components of package’s SDK, which is specified in its ``daml.yaml``
- Overlay ``override-components`` of ``multi-project.yaml``
- Overlay ``override-components`` of ``daml.yaml``, so effectively this has the highest precedence in case of conflicts

Components specified in ``override-components`` must be installed by running

.. code:: shell

    dpm install package

in a package containing the ``daml.yaml`` or ``multi-package.yaml``
