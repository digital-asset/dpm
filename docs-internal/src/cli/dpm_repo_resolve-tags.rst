Dpm Repo Resolve-Tags
=====================

.. _dpm_repo_resolve-tags:

dpm repo resolve-tags
---------------------

resolve the tag of one or more components to corresponding (semantic) versions

Synopsis
~~~~~~~~



Resolve the tag (e.g. 'latest') of one or more components to corresponding (semantic) versions.

Components can be passed directly as cli arguments. 
Alternatively, you can pass the config file used with "dpm repo create-tarball"

...
components:
  foo:
    image-tag: latest
  bar:
    version: 1.2.3-whatever
  baz:
    image-tag: some-tag
assistant:
  image-tag: latest

The output will be the same content, but with components that have "image-tag" replaced with resolved versions.


::

  dpm repo resolve-tags <component>:<tag>...<component>:<tag> [flags]

Options
~~~~~~~

::

      --auth string                  path to a config file similar to docker’s config.json to use for authenticating to the OCI registry. Defaults to docker's config.json
      --from-publish-config string   resolve component tags in publish.yaml file
  -h, --help                         help for resolve-tags
      --insecure                     use http instead of https for OCI registry
      --registry string              OCI registry to use for pushing

SEE ALSO
~~~~~~~~

* :ref:`dpm repo <dpm_repo>` 	 - 

