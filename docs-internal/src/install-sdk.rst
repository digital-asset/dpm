Installing the SDK
==================

When installing (any edition, and for any platform), you can set the
``${DPM_HOME}`` environment variable to change the location
where the SDK and any future updates are installed. The default is
``${HOME}/.dpm/``.

You should then add ``${HOME}/.dpm/bin`` to your ``PATH``:

.. code:: shell

   export PATH="${HOME}/.dpm/bin:${PATH}"

or, if you used a custom install location:

.. code:: shell

   export PATH="${DPM_HOME}/bin:${PATH}"

Open-source SDK
~~~~~~~~~~~~~~~

To install latest version: - mac/linux:

.. code:: shell

   curl https://get.digitalasset.com/install/install.sh | sh

-  windows:

Download and run the `installer <https://get.digitalasset.com/install/latest-windows.html>`_, which will install the dpm sdk and set up the PATH variable for you.

Upgrading the SDK
-----------------

You can upgrade to the latest version of an SDK by running:

.. code:: shell

   dpm install latest

This will install the latest SDK of the same edition you have installed
already.

(you won’t be required to provide credentials again, since you should
already be configured as part of installing the SDK the first time)

You can also install a specific version of the SDK in a similar way:

::

   dpm install <version>

Note that outside of a daml project, the active SDK will be the latest
version that’s installed.
