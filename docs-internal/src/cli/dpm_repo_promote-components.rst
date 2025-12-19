Dpm Repo Promote-Components
===========================

.. _dpm_repo_promote-components:

dpm repo promote-components
---------------------------

re-publish components from one OCI registry (public-unstable) to another (public)

Synopsis
~~~~~~~~


re-publish components from one OCI registry (public-unstable) to another (public)

::

  dpm repo promote-components [flags]

Examples
~~~~~~~~

::

    dpm repo promote-components --source-registry=gar.io/public-unstable --destination-registry=gar.io/public -f publish.yaml

Options
~~~~~~~

::

      --auth string                   path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json
  -f, --config-file string            REQUIRED config file path"
      --destination-registry string   destination OCI registry to publish components to
  -h, --help                          help for promote-components
      --insecure                      use http instead of https for OCI registry
      --oci-cache string              use an oci-cache to speed up pulls
      --source-registry string        source OCI registry to pull components from

SEE ALSO
~~~~~~~~

* :ref:`dpm repo <dpm_repo>` 	 - 

