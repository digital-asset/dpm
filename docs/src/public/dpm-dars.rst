.. _dpm-custom-dars:

Publishing Dars
*********************

.. note::

    This functionality is available in DPM version 1.0.12 or later (or bundled with SDK 3.5 or later)

To share or use your Dars in various projects, you can publish it to an OCI repository.

.. code:: shell

    dpm publish dar  \
        --name=<dar name> \
        --version=<dar strict semantic version> \
        --file=<path to dar file> \
        --registry oci://<location to publish to>

for example:

.. code:: shell

    dpm publish dar \
        --name=foo \
        --version=1.0.0 \
        --file=bar/foo.dar \
        --registry oci://example.com/my/dars

This will publish version ``1.0.0`` of ``foo`` as OCI to ``example.com/my/components/foo:1.0.0`` using the dar located at ``bar/foo.dar```

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
