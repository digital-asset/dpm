{ pkgs, self }:

let 
  # taken from: https://nix.dev/manual/nix/2.24/command-ref/new-cli/nix3-flake#flake-reference-attributes
  commitSha = if self ? rev then self.rev else "unknown";
  commitDate = if self ? lastModifiedDate then builtins.toString self.lastModifiedDate else "unknown";
in

pkgs.buildGoModule {
  name = "dpm";
  doCheck = false;
  src = self;
  vendorHash = "sha256-SoiT/+VLmIZQ4qsrLDY6eIQ00i55RsKyUWgZDxgtFlY=";

  ldflags = [
    "-X daml.com/x/assistant/pkg/assistantversion.Build=${commitSha}"
    "-X daml.com/x/assistant/pkg/assistantversion.BuildDate=${commitDate}"
  ];
}
