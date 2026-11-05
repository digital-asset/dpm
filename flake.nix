{
  inputs = {
    nixpkgs.url = "nixpkgs/nixos-26.05";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem
      (system:
        let
          pkgs = import nixpkgs {
            inherit system;
            overlays = import ./nix/overlays.nix;
          };
          assistant = import ./hosted.nix  { inherit pkgs ; inherit self; };
        in
        {
          packages.assistant = assistant;
          devShells.default = import ./nix/shell.nix { inherit pkgs; ci = false; };
          devShells.ci = import ./nix/shell.nix { inherit pkgs; ci = true; };
        }
      );
}
