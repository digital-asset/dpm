Development and testing of components
=====================================

The assistant makes it easy to develop and test integrating (multiple)
:doc:`components <./components>` locally, using the
``dpm component init`` :doc:`command <../cli/dpm_component_init>`

This will create two yaml files in the current directory: -
component.yaml: lets you define the commands that your
component-under-development exposes as part of ``dpm`` when this
component is imported - dpm.local.yaml: this tells ``dpm`` to
“import” the components defined in this file. This importing happens by
simply running ``dpm``.

.. code:: yaml

   # dpm.local.yaml

   override-components:
     my-local-component:
       # this is essentially pointing to component.yaml that was created in the same directory
       local-path: .

.. code:: yaml

   # component.yaml

   apiVersion: digitalasset.com/v1
   kind: Component
   spec:
     commands:
     #    - path:
     #      name:
     #      desc:
     #      exec-args: []
     #      aliases: []

     jar-commands:
   #    - path:
   #      name:
   #      desc:
   #      jar-args: []
   #      jvm-args: []
   #      aliases: []

You can now edit ``component.yaml`` to add commands as desired (see
:doc:`components docs <./components>` for more details).

importing multiple components
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

You can modify ``dpm.local.yaml`` to have the assistant import
additional components. These additional components can reside locally,
or can be remote ones that reside in some OCI registry:

.. code:: yaml

   # dpm.local.yaml

   override-components:
     my-local-component:
       local-path: .

     foo:  # remote component
       version: 1.2.3 

     bar:  # another local component
       local-path: ../project/bar

Now when you run:

::

   dpm --help

the assistant will automatically import the two local components, and
the remote one too! The commands defined in them will be incorporated
into ``dpm``, and displayed as part of the help message.

**Note/warning**: the assistant will ignore ``daml.yaml`` and any
installed dpm-sdks when it sees a ``dpm.local.yaml`` in the working
directory!

Overriding and extending an SDK’s components locally
----------------------------------------------------

While ``dpm.local.yaml`` allows side-stepping SDKs completely to allow
component development in absence of any installed SDKs, the assistant
also supports extending and overriding an installed SDK’s components in
the context of daml single and multi-package projects.

in ``daml.yaml``
~~~~~~~~~~~~~~~~

You can use the same ``override-components`` yaml object (described
above) in ``daml.yaml``. You can use this to: - import additional
components present locally - import additional components present
remotely (in some OCI repo) - replace a component that’s provided by an
SDK with one present locally - replace a component that’s provided by an
SDK with one present remotely (in some OCI repo)

.. code:: yaml

   sdk-version: 4.5.6
   ...
   override-components:
     foo:  # additional component (remote)
       version: 1.2.3

     bar:  # additional component (local)
       local-path: ../projects/bar
       
     damlc: # replaces "damlc" component that's part of the chosen sdk-version 4.5.6 
       local-path: ../projects/my-damlc

     scribe: # replaces "scribe" component that's part of the chosen sdk-version 4.5.6 
       version:  1.2.3   # remote

   ...

in ``multi-package.yaml``
~~~~~~~~~~~~~~~~~~~~~~~~~

You can use the same ``override-components`` yaml object in a
``multi-package.yaml`` too. This applies the specified overrides to all
packages.

.. code:: yaml

   packages:
     - ./daml-pkg-1
     - ./daml-pkg-2

   override-components:
     foo:  # additional component (remote)
       version: 1.2.3

     bar:  # additional component (local)
       local-path: ../projects/bar

     damlc: # replaces "damlc" component if it's part of a given project's SDk, or adds it to it otherwise
       local-path: ../projects/my-damlc

     scribe: # replaces "scribe" component if it's part of a given project's SDk, or adds it to it otherwise
       version:  1.2.3   # remote

When both ``multi-project.yaml`` and one of its project’s ``daml.yaml``
simultaneously override components, the assistant overlays (i.e. merges)
``daml.yaml``\ ’s components on top of ``multi-project.yaml``\ ’s
components. So the overall effective components for that project are
determined like this: - Start with components as defined by assembly
manifest of the project’s ``daml.yaml``\ ’s chosen sdk - Overlay
``override-components`` of ``multi-project.yaml`` - Overlay the
project’s ``daml.yaml``\ ’s override-components (so effectively this has
the highest precedence in case of conflicts)
