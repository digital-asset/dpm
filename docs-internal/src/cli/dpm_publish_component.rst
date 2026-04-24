Dpm Publish Component
=====================

.. _dpm_publish_component:

dpm publish component
---------------------

Publish a component to an OCI registry

Synopsis
~~~~~~~~


Will publish the component (OCI index) to <registry>/<name>:<version>

::

  dpm publish component registry [flags]

Examples
~~~~~~~~

::

  dpm artifacts publish component 'oci://whatever.dev/bar/test/foo:1.2.3-alpha' -p linux/amd64=dist/foo-linux -p darwin/arm64=dist/foo-darwin 

Options
~~~~~~~

::

  -a, --annotations stringToString   annotations to include in the published OCI artifact (default [])
      --auth string                  path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json
  -d, --dry-run                      don't actually push to the registry
  -t, --extra-tags strings           publish extra tags besides the semver
  -h, --help                         help for component
  -g, --include-git-info             include git info as annotations on the published manifest
      --insecure                     use http instead of https for OCI registry
  -p, --platform stringToString      REQUIRED <os>/<arch>=<path-to-component> or generic=<path-to-component> (default [])

SEE ALSO
~~~~~~~~

* :ref:`dpm publish <dpm_publish>` 	 - Commands for publishing artifacts

