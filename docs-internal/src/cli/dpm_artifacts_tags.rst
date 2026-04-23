Dpm Artifacts Tags
==================

.. _dpm_artifacts_tags:

dpm artifacts tags
------------------

list published tags of an artifact

Synopsis
~~~~~~~~


List all tags associated with an artifact (dar/component) at an arbitrary OCI registry

::

  dpm artifacts tags [flags]

Examples
~~~~~~~~

::

  dpm artifacts list --artifact 'oci://whatever.dev/bar/test'

Options
~~~~~~~

::

      --auth string   path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json
  -h, --help          help for tags
  -n, --name string   full uri of artifact to search for

SEE ALSO
~~~~~~~~

* :ref:`dpm artifacts <dpm_artifacts>` 	 - Commands for managing artifacts

