{
  inputs = {
    nixpkgs.url = "nixpkgs/nixos-25.11";
    flake-utils.url = "github:numtide/flake-utils";
    nixpkgs-2411.url = "nixpkgs/nixos-24.11";
  };

  outputs = { self, nixpkgs, nixpkgs-2411, flake-utils }:
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
