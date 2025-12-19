Getting started
===============

Installing ``dpm``
--------------------

You can obtain the assistant in two ways:
 * :doc:`install a full dpm-sdk (tarball) <./install-sdk>`
 * download the latest bare ``dpm`` binary from `the public repo <https://console.cloud.google.com/artifacts/docker/da-images/europe/public/components%2Fdpm?invt=Abt4Ag&project=da-images>`__ using ``oras`` or similar tool:
    .. code:: shell

       ## use oras cli to pull the correct platform image:
       oras pull --platform darwin/arm64 europe-docker.pkg.dev/da-images/public/components/dpm:latest

       ## make the binary executable:
       chmod +x dpm

See the :doc:`component-development <components/component-dev>` docs
next.
