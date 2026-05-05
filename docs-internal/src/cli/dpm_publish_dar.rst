Dpm Publish Dar
===============

.. _dpm_publish_dar:

dpm publish dar
---------------

Publish a dar to an OCI registry

Synopsis
~~~~~~~~


Publish a dar to an OCI registry

::

  dpm publish dar <registry> [flags]

Examples
~~~~~~~~

::

  dpm artifacts publish dar 'oci://whatever.dev/bar/test/foo:1.2.3-alpha' -f path/to/foo.dar

Options
~~~~~~~

::

  -a, --annotations stringToString   annotations to include in the published OCI artifact (default [])
      --auth string                  path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json
  -d, --dry-run                      don't actually push to the registry
  -t, --extra-tags strings           publish extra tags besides the semver
  -f, --file string                  REQUIRED path to the dar file to publish
  -h, --help                         help for dar
  -g, --include-git-info             include git info as annotations on the published manifest
      --insecure                     use http instead of https for OCI registry

SEE ALSO
~~~~~~~~

* :ref:`dpm publish <dpm_publish>` 	 - Commands for publishing artifacts

