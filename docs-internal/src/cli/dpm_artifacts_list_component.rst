Dpm Artifacts List Component
============================

.. _dpm_artifacts_list_component:

dpm artifacts list component
----------------------------

list published tags of a component

Synopsis
~~~~~~~~


Will list all tags associated with a component at an arbitrary OCI registry

::

  dpm artifacts list component [flags]

Examples
~~~~~~~~

::

  dpm artifacts list component --name foo --registry 'oci://whatever.dev/bar/test'

Options
~~~~~~~

::

  -h, --help              help for component
  -n, --name string       name of component to search for
      --registry string   OCI registry to search in

SEE ALSO
~~~~~~~~

* :ref:`dpm artifacts list <dpm_artifacts_list>` 	 - Commands for listing artifacts

