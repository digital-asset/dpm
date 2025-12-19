Dpm Repo Publish-Sdk-Manifest
=============================

.. _dpm_repo_publish-sdk-manifest:

dpm repo publish-sdk-manifest
-----------------------------

publish an sdk's manifest

Synopsis
~~~~~~~~


Creates, validates and then publishes an sdk manifest to OCI registry

::

  dpm repo publish-sdk-manifest [flags]

Examples
~~~~~~~~

::

    dpm repo publish-sdk-manifest --registry=gar.io/foo-org -f publish.yaml

Options
~~~~~~~

::

      --auth string          path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json
  -f, --config-file string   REQUIRED config file path"
  -t, --extra-tags strings   publish extra tags besides the semver
  -h, --help                 help for publish-sdk-manifest
      --insecure             use http instead of https for OCI registry
      --oci-cache string     use an oci-cache to speed up pulls
      --registry string      OCI registry to use for pulling/pushing

SEE ALSO
~~~~~~~~

* :ref:`dpm repo <dpm_repo>` 	 - 

