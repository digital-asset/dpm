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

Additionally, DPM (and DPM's installation process) make use of the system's default temp directory.
On Unix systems, it uses ``$TMPDIR`` if non-empty, else ``/tmp``.
On Windows, it uses ``GetTempPath``, returning the first non-empty value from ``%TMP%``, ``%TEMP%``, ``%USERPROFILE%``, or the ``Windows`` directory.

Project Configuration
*********************

.. _dpm-configuration-files:

``dpm`` uses ``daml.yaml`` and ``multi-package.yaml`` for single and multi-package project configuration.


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


.. note::

    This functionality is available in DPM version 1.0.14 or later (or bundled with SDK 3.5 or later)

You can also specify the `sdk-version:` field in ``multi-package.yaml``. This SDK version applies to all packages in the multi-package, unless a package's individual ``daml.yaml`` specifies its own `sdk-version`, which takes precedence over the one in ``multi-package.yaml``.

.. code-block:: yaml

  sdk-version: 3.4.11
  packages:
    - ./path/to/package/a
    - ./path/to/package/b

The multi-package also includes an optional ``dars`` field, for providing additional information to Daml Studio.
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


.. note::

    This functionality is available in DPM version 1.0.14 or later (or bundled with SDK 3.5 or later)

Additionally, if you specify the `sdk-version` in the `multi-package.yaml` that references your project `daml.yaml`, you can exclude
repeating the `sdk-version:` field in your `daml.yaml`, and the value specified in your `multi-package.yaml` will be used.

Additionally, you can avoid specifying an sdk-version entirely and only opt-in to particular components, as outlined in the examples below.

.. _dpm-override-components:

Opt-in Components
-----------------

.. note::

    This functionality is available in DPM version 1.0.14 or later (or bundled with SDK 3.5 or later)


``dpm`` supports opting in to the components in a single and/or a multi-package project instead of relying on an sdk-version bundle.
You can use pre-existing components, or ones you create and publish (see :ref:`these docs <dpm-custom-components>` on publishing your own components).

in single-package projects
~~~~~~~~~~~~~~~~~~~~~~~~~~

.. code:: yaml

   # daml.yaml

   components:
     # damlc to use version 3.5.1-rc1
     - damlc:3.5.1-rc1

     # adding component "foo"
     - oci://example.com/some/path/foo:1.2.3

     # codegen-java to use a component present locally on the filesystem
     - name: codegen-java
       path: ../path/to/component/directory

in multi-package projects
~~~~~~~~~~~~~~~~~~~~~~~~~

For multi-package projects, you must specify the ``components`` yaml object in ``multi-package.yaml``. This applies the specified components to all packages.

.. code:: yaml

   # multi-package.yaml

   packages:
     - ./daml-pkg-1
     - ./daml-pkg-2

   components:
     - damlc:3.5.1-rc1

     # adding component "foo"
     - oci://example.com/some/path/foo:1.2.3

     # component present locally on the filesystem
     - name: codegen-java
       path: ../path/to/component/directory

When a ``daml.yaml`` in a multi-package project also defines the `components` field, ``dpm`` gives precedence to ``daml.yaml`` components over the values specified in ``multi-package.yaml``.

Components specified in ``components`` must be installed by running

.. code:: shell

    dpm install package

in a directory containing the ``daml.yaml`` or ``multi-package.yaml``

.. warning::

    When using this component opt-in feature, you should not specify ``sdk-version`` field in either your ``multi-package.yaml`` or ``daml.yaml`` files.  Otherwise, you might get unintended results.

.. note::

    Beginning with SDK version 3.5, the ``override-components`` field has been removed in favor of the ``components`` field.
