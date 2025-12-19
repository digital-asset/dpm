Components and component.yaml
=============================

Components are artifacts that the assistant (being primarily a
*launcher*) wraps. Examples of components are ``sandbox`` and
``scribe``. A component defines the commands it wants the assistant to
expose, as well as the binaries that back these commands.

A component is a directory that contains: - a ``component.yaml`` file at
its root: The main utility of this is to tell the assistant how to
import the component’s commands and ``exec`` them - binary(s) or JAR(s)
that back the commands defined in ``component.yaml`` - optionally:
additional arbitrary files and directories (if desired/necessary for the
operation of the component, e.g. ``lib`` directories and such). When the
component is published, the assistant will include these in the artifact
(see :doc:`publishing components <./publishing-components>`)

Components, or rather the commands in them, can be backed by native
binaries and/or JARs. This is a sample component.yaml defining two
commands (``foo`` and ``bar``) backed by the same native binary:

.. code:: yaml

   spec:
     commands:
       - path: ./foo-binary
         name: foo
         aliases: ["f"]
         desc: a command for foo'ing!
       - path: ./foo-binary
         name: bar
         aliases: ["b"]
         desc: a command for bar'ing!
         exec-args: ["--should-bar-instead-of-foo"]

When running ``dpm --help``, you’ll see the following in the help
message:

.. code:: shell

   Usage:
     dpm [command]
   ...
   ...
   Dpm-SDK Commands
     foo    a command for foo'ing!
     bar    a command for bar'ing!
   ...

The ``exec-args`` on the ``bar`` command tells the assistant to supply
additional args when running the binary backing the command, so it’s
executed as: ``./foo-binary --should-bar-instead-of-foo``.

To start developing your own component see the :doc:`component development <./component-dev>` docs section.

Dependency on other components
------------------------------

You can optionally declare dependencies of your component on other
components:

.. code:: yaml

   # component.yaml

   dependency-paths:
     foo: CUSTOM_FOO_PATH
   spec:
     commands:
       ...

In this example, this component is expecting a component named ``foo``
to be present when the running the assistant. The assistant will inject
an environment variable named ``CUSTOM_FOO_PATH`` into this component’s
env, which will point to the path of ``foo`` on the filesystem.

When an SDK is assembled and published, the release process (utilizing
the assistant) will make sure that dependencies are present, otherwise,
the SDK assembly will fail.

When developing components, the developer is responsible for ensuring
the assistant brings in the dependencies, so the dependency components
must be present either as part of the installed SDK, or in ``daml.yaml``
or in ``dpm.local.yaml`` (See :doc:`component development docs <./component-dev>` for how to import additional components on
top of, or without, an SDK)

component.yaml schema
---------------------

See the
`json-schema <https://github.com/DACH-NY/dpm/blob/main/schema/component.schema.json>`__
