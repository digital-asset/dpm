Frequently-asked Questions
==========================

I published a component to an OCI repo, how do I test pulling it with ``dpm``?
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

There are many ways in which the assistant pulls remote components.

For quick sanity-checking a published component:

.. code:: shell

   export DPM_REGISTRY=<REGISTRY>
   dpm component run \
     <component name> <component version> \
     [desired compoennt command] [args to pass to that component command]

e.g. 

.. code:: shell

   export DPM_REGISTRY=europe-docker.pkg.dev/da-images/public
   dpm component run canton-open-source 3.3.0-snapshot.20250429.15795.0.v6b2dcccb

The assistant will pull your component into
``~/.dpm/cache/components/``, and then run the specified command of
your component

Other ways in which you can pull and run specific remote components are:
 * using ``override-components`` in ``dpm.local.yaml``
 * using ``override-components`` in ``daml.yaml``
 * using ``override-components`` in ``multi-package.yaml``

For more details on these, see the
:doc:`component-development <./components/component-dev>` docs.
