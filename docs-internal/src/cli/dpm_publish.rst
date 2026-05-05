Dpm Publish
===========

.. _dpm_publish:

dpm publish
-----------

Command for publishing an artifact to an OCI registry

Synopsis
~~~~~~~~


Command/subcommands for publishing artifacts to an OCI registry

::

  dpm publish <registry> [flags]

Options
~~~~~~~

::

  -a, --annotations stringToString   annotations to include in the published OCI artifact (default [])
      --auth string                  path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json
  -d, --dry-run                      don't actually push to the registry
  -t, --extra-tags strings           publish extra tags besides the semver
  -h, --help                         help for publish
  -g, --include-git-info             include git info as annotations on the published manifest
      --insecure                     use http instead of https for OCI registry
  -p, --platform stringToString      REQUIRED <os>/<arch>=<path-to-component> or generic=<path-to-component> (default [])

SEE ALSO
~~~~~~~~

* :ref:`dpm <dpm>` 	 - 
* :ref:`dpm publish component <dpm_publish_component>` 	 - Publish a component to an OCI registry

