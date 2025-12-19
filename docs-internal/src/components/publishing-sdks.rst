Publishing SDKs
===============

The assistant aids in the SDK assembly and release process. This is
mainly a use-case for the
`dpm-assembly <https://github.com/DACH-NY/dpm-assembly>`__ repo.

Publishing SDKs typically involves publishing:
 * a standalone sdk “fat” tarball (per supported platform) for use with dpm bootstrap. Components chosen to be part of the SDK will automatically be pulled. The assistant won’t handle publishing the tarball though.
 * an assembly manifest to the OCI registry for use with dpm install Components’ release and publishing lifecycle is independent, and components are expected to be published to OCI registry prior to publishing an SDK.

See:
 * ``dpm repo publish-sdk-manifest`` :doc:`command <../cli/dpm_component_init>`
 * ``dpm repo create-tarball`` :doc:`command <../cli/dpm_component_init>`
