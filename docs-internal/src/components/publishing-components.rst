Publishing components
=====================

The assistant fully handles packaging and publishing
:doc: `components <./components>`. Components are published as
`OCI <https://opencontainers.org/>`__ artifacts to an OCI registry,
mainly `the public-unstable OCI registry <https://console.cloud.google.com/artifacts/docker/da-images/europe/public-unstable?invt=Abt4Ag&project=da-images>`__.

As a component developer, you can use the
``dpm repo publish-component``
:doc:`command <../cli/dpm_repo_publish-component>` to publish a
component. This is platform aware, and the command requires the target
platform(s) be specified:

::

     --platform darwin/arm64=<path to prepared directory containg the component for this platform>

Multiple platforms can be published in a single command by providing
multiple ``--platform``\ ’s. For JAR components, you can use
``--platform generic=<path to dir>``

Here’s a sample component ``foo``:

.. code:: shell

   ./
     darwin-arm64/
           component.yaml
           foo              # darwin/arm64 binary
           foo2             # another binary
           lib/...          # directory
           somefile.txt
     linux-amd64/
           component.yaml
           foo
           lib/...

The ``component.yaml`` file is required, and must be present at the root
of the directory (of each platform, if there are multiple!)

We can publish it like this:

::

   dpm repo component-publish \
     foo \                  # name of the component 
     1.2.3-pre-alpha \      # version (semantic version)
     --platform darwin/arm64=./darwin-arm64 \
     --platform linux/amd64=./linux-amd64 \
     --registry <destination OCI registry> \
     --extra-tags latest    # also pushing a 'latest' tag for this version of the component  

This will run some validations, then publish two `OCI
images <https://specs.opencontainers.org/image-spec/manifest/#image-manifest>`__
corresponding to the two platforms, and an `OCI
index <https://specs.opencontainers.org/image-spec/image-index/?v=v1.0.1>`__,
plus the ``latest`` tag we specified.

The published artifact (for a given platform) will include everything
that’s present in the given directory (for that platform). So, it will
include all the files and subdirectories (e.g. ``lib``) in it! You don’t
have to worry about pre-bundling or zipping up anything. You just
provide the raw directory(s). When pulling a component, the assistant
will handle restoring the artifact exactly as given at the time of
publishing.

The component can now be imported (i.e. pulled) and run by dpm.
See this :ref:`FAQ question <faq:i published a component to an oci repo, how do i test pulling it with \`\`dpm\`\`?>`
on how to pull components.

Annotations
~~~~~~~~~~~

When publishing build artifacts you can also apply annotations to the
entire image. The ``-a`` or ``--annotations`` can be used with a key
value pair multiple times to add annotations to the image that is being
published.

This can be useful to apply/provide metadata to the image such as what
branch the build artifacts were built from, the commit sha related to
the artifacts themselves.
