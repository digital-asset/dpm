Dpm Repo Publish-Dpm
====================

.. _dpm_repo_publish-dpm:

dpm repo publish-dpm
--------------------

Publish the assistant to an OCI registry

Synopsis
~~~~~~~~


Publish the assistant to an OCI registry

::

  dpm repo publish-dpm <version> [flags]

Examples
~~~~~~~~

::

    dpm repo publish-dpm 1.2.3-alpha -p linux/arm64=dist/dpm -p windows/amd64=dist/dpm.exe

Options
~~~~~~~

::

  -a, --annotations stringToString   annotations to include in the published OCI artifact (default [])
      --auth string                  path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json
  -d, --dry-run                      don't actually push to the registry
  -t, --extra-tags strings           publish extra tags besides the semver
  -h, --help                         help for publish-dpm
  -g, --include-git-info             include git info as annotations on the published manifest
      --insecure                     use http instead of https for OCI registry
  -p, --platform stringToString      REQUIRED <os>/<arch>=<path-to-assistant's-binary> (default [])
      --registry string              OCI registry to use for pushing

SEE ALSO
~~~~~~~~

* :ref:`dpm repo <dpm_repo>` 	 - 

