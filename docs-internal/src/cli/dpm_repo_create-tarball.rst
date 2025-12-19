Dpm Repo Create-Tarball
=======================

.. _dpm_repo_create-tarball:

dpm repo create-tarball
-----------------------

create an sdk tarball(s) for one or more platforms

Synopsis
~~~~~~~~


Pulls down components (including the assistant) from the OCI registry, then dumps out and validates an sdk bundle (for each specified platform) 

::

  dpm repo create-tarball [flags]

Examples
~~~~~~~~

::

    dpm repo create-tarball --registry=gar.io/foo-org -f publish.yaml

Options
~~~~~~~

::

      --auth string          path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json
  -f, --config-file string   REQUIRED config file path"
  -h, --help                 help for create-tarball
      --insecure             use http instead of https for OCI registry
      --oci-cache string     use an oci-cache to speed up pulls
  -o, --output string        output path of the bundle (default ".")
      --registry string      OCI registry to use for pulling/pushing

SEE ALSO
~~~~~~~~

* :ref:`dpm repo <dpm_repo>` 	 - 

