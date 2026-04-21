Dpm Artifacts Publish Component
===============================

.. _dpm_artifacts_publish_component:

dpm artifacts publish component
-------------------------------

Publish a component to an OCI registry

Synopsis
~~~~~~~~


Will publish the component (OCI index) to <registry>/<name>:<version>

::

  dpm artifacts publish component [flags]

Examples
~~~~~~~~

::

  dpm artifacts publish component --name foo --version 1.2.3-alpha -p linux/amd64=dist/foo-linux -p darwin/arm64=dist/foo-darwin --registry 'oci://whatever.dev/bar/test'

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
  -n, --name string                  name of component to be pushed
  -p, --platform stringToString      REQUIRED <os>/<arch>=<path-to-component> or generic=<path-to-component> (default [])
      --registry string              OCI registry to use for pushing
  -v, --version string               version of component to be pushed

SEE ALSO
~~~~~~~~

* :ref:`dpm artifacts publish <dpm_artifacts_publish>` 	 - Commands for publishing artifacts

