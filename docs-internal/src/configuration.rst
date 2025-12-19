Configuration
=============

DPM_HOME
--------------------

This environment variable allows isolating multiple installations and
configurations of an SDK (or just ``dpm`` itself) on a system.
Defaults to ``~/.dpm``.

Configuration options
---------------------

The assistant can be configured via (simultaneously): - config file:
``${DPM_HOME}/dpm-config.yaml`` - environment variables:
which take precedence over fields in ``dpm-config.yaml``, and so can
be used to override.

For a listing of all available configuration settings, see
`here <https://github.com/DACH-NY/dpm/blob/main/pkg/assistantconfig/envvars.go>`__

Registry and Edition
--------------------

For many of its built-in commands (except ``dpm repo`` commands), the
assistant needs to know which OCI registry is in effect. If
the assistant was obtained as part of an SDK, these will automatically
be set in ``${DPM_HOME}/dpm-config.yaml`` (where
``${DPM_HOME}`` is usually ``~/.dpm``)

However, if the assistant being used is the bare binary obtained
directly from its OCI repo, these configurations might not be known. You
can manually set them either in
``${DPM_HOME}/dpm-config.yaml``:

.. code:: yaml

   registry: europe-docker.pkg.dev/da-images/public # or desired oci registry

or using the ``DPM_REGISTRY`` environment variables.

Authentication
--------------

Authentication (for pulling and pushing to/from OCI registries) is based
on a file similar to ``~/.docker/config.json``, though a docker
installation on the system isn’t necessary.

The assistant defaults to using the docker’s auth. You can configure
docker to authenticate to the OCI registry (Google Artifact Registry),
and the assistant will use that.

.. code:: shell

   gcloud auth login

.. code:: shell

   gcloud auth configure-docker europe-docker.pkg.dev
