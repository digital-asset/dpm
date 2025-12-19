Dpm Login
=========

.. _dpm_login:

dpm login
---------



Synopsis
~~~~~~~~


Authenticate to the registry. This will modify the auth config file. (The registry and auth file are the ones specified in dpm-config.yaml or the corresponding env vars, or the defaults if none are set)

::

  dpm login [flags]

Options
~~~~~~~

::

  -h, --help                    help for login
  -n, --netrc string            log in using username and password of a netrc host (machine)
  -p, --password string         password
      --password-stdin          Take the password from stdin
      --use-native-cred-store   store credentials in system's credential store instead of plaintext in the auth config file
  -u, --username string         username

SEE ALSO
~~~~~~~~

* :ref:`dpm <dpm>` 	 - 

