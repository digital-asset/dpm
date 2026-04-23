Dpm Artifacts Tags
==================

.. _dpm_artifacts_tags:

dpm artifacts tags
------------------

list published tags of an artifact

Synopsis
~~~~~~~~


Will list all tags associated with an artifact (dar/component) at an arbitrary OCI registry

::

  dpm artifacts tags [flags]

Examples
~~~~~~~~

::

  dpm artifacts list --name foo --registry 'oci://whatever.dev/bar/test'

Options
~~~~~~~

::

  -h, --help              help for tags
  -n, --name string       name of component to search for
      --registry string   OCI registry to search in

SEE ALSO
~~~~~~~~

* :ref:`dpm artifacts <dpm_artifacts>` 	 - Commands for managing artifacts

