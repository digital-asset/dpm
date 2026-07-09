{ pkgs, pkgs2411, ci }:
let
  requiredPackages = with pkgs; ([
    # these packages are required both in CI and for local development
      bash
      gh
      jq
      openjdk21
      sbt
      zip
  ] ++ (if ci then [
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
