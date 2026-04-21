.. _dpm-custom-components:

Publishing Components
*********************

.. note::

    this is available in SDK 3.5 or later

To share or use your Component in various projects, you can publish it to a repository.

.. code:: shell

    dpm artifacts publish component oci \
        <component name> <component strict semantic version> \
        --platform generic="/path/to/component/directory" \
        --registry <location to publish to>

for example:

.. code:: shell

    dpm artifacts publish component oci \
        foo 1.0.0 \
        --platform generic="~/component-foo" \
        --registry example.com/my/components

This will publish  version ``1.0.0`` of ``foo`` as OCI to ``example.com/my/components/foo:1.0.0``

For multi-platform components, you can instead provide a directory for each platform.
For example:

.. code:: shell

    dpm artifacts publish component oci \
        foo 1.0.0 \
        --platform linux/arm64="/some/directory" \
        --platform windows/amd64="/another/directory" \
        --registry example.com/my/components

See the ``dpm artifacts publish components --help`` command for more available options.

For information on how to use this in your project, see the section on :ref:`using components <dpm-override-components>`
