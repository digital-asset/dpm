.. _dpm-artifact-tags:

List Artifact Tags
*********************

.. note::

    This functionality is available in DPM version 1.0.12 or later (or bundled with SDK 3.5 or later)

To view an artifact (component/dar) that has published to an OCI registry, you can list the tags associated with that artifact

.. code:: shell

    dpm tags <uri_to_artifact>

for example:

.. code:: shell

    dpm tags oci://europe-docker.pkg.dev/da-images/public/foo/meep

This will list all the tags of the ``meep`` artifact under the public/foo repository in the europe-docker.pkg.dev/da-images
registry

See the ``dpm tags --help`` command for more available options.
