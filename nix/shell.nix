{ pkgs, pkgs2411, ci }:
let
  requiredPackages = with pkgs; ([
    # these packages are required both in CI and for local development
      bash
      go
      go-junit-report
      gotools
      goreleaser
      gnumake
      jq
      pkgs2411.google-cloud-sdk
      zip
      (python3.withPackages (pkgs: [ pkgs.sphinx pkgs.sphinx-rtd-theme]))
  ] ++ (if ci then [
    # these packages should only be installed on CI
    openjdk17
  ] else [
    # these packages are only installed on developer machines locally
    circleci-cli
    oras
    pandoc
  ])) ++ (lib.optionals stdenv.isDarwin [
          pkgs.libiconv
          ]);
in
pkgs.mkShell {
  packages = requiredPackages;
  shellHook = ''
    # there is a nix bug that the directory deleted by _nix_shell_clean_tmpdir can be the same as the general $TEMPDIR
    eval "$(declare -f _nix_shell_clean_tmpdir | sed 's/_nix_shell_clean_tmpdir/orig__nix_shell_clean_tmpdir/')"
    _nix_shell_clean_tmpdir() {
        orig__nix_shell_clean_tmpdir "$@"
        mkdir -p "$TEMPDIR" # ensure system TEMPDIR still exists
    }
    '';
}
