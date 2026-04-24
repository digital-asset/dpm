.. _dpm-custom-components:

Publishing Components
*********************

.. note::

    This functionality is available in DPM version 1.0.12 or later (or bundled with SDK 3.5 or later) 

To share or use your Component in various projects, you can publish it to a repository.

.. code:: shell

    dpm publish component  \
        --name=<component name> \
        --version=<component strict semantic version> \
        --platform generic="/path/to/component/directory" \
        --registry oci://<location to publish to>

for example:

.. code:: shell

    dpm publish component \
        --name=foo \
        --version=1.0.0 \
        --platform generic="~/component-foo" \
        --registry oci://example.com/my/components

This will publish version ``1.0.0`` of ``foo`` as OCI to ``example.com/my/components/foo:1.0.0``

For multi-platform components, you can instead provide a directory for each platform.
For example:

.. code:: shell

    dpm publish component \
        --name=foo \
        --version=1.0.0 \
        --platform linux/arm64="/some/directory" \
        --platform windows/amd64="/another/directory" \
        --registry oci://example.com/my/components

See the ``dpm publish component --help`` command for more available options.

For information on how to use this in your project, see the section on :ref:`using components <dpm-override-components>`
