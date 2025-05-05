{
  description = "Nix packages";
  inputs = {
    # keep-sorted start block=yes case=no
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable-small";
    pre-commit-hooks = {
      inputs.nixpkgs.follows = "nixpkgs";
      url = "github:cachix/git-hooks.nix";
    };
    systems.url = "github:nix-systems/default";
    treefmt-nix = {
      inputs.nixpkgs.follows = "nixpkgs";
      url = "github:numtide/treefmt-nix";
    };
    ascii-pkgs.url = "github:a1994sc/nix-pkgs";
    flake-utils = {
      inputs.systems.follows = "systems";
      url = "github:numtide/flake-utils";
    };
    # keep-sorted end
  };
  nixConfig = {
    extra-substituters = [
      "https://a1994sc.cachix.org"
    ];
    extra-trusted-public-keys = [
      "a1994sc.cachix.org-1:xZdr1tcv+XGctmkGsYw3nXjO1LOpluCv4RDWTqJRczI="
    ];
  };
  outputs =
    inputs@{ self, nixpkgs, ... }:
    inputs.flake-utils.lib.eachSystem
      [
        "x86_64-linux"
        "aarch64-linux"
      ]
      (
        system:
        let
          pkgs = import nixpkgs {
            inherit system;
          };
          fmt = inputs.treefmt-nix.lib.evalModule pkgs (
            { pkgs, ... }:
            {
              # keep-sorted start block=yes
              programs.keep-sorted.enable = true;
              programs.nixfmt = {
                enable = true;
                package = pkgs.nixfmt-rfc-style;
              };
              projectRootFile = "flake.nix";
              # keep-sorted end
            }
          );
        in
        {
          devShells.default = pkgs.mkShell {
            shellHook = self.checks.${system}.pre-commit-check.shellHook;
            buildInputs = self.checks.${system}.pre-commit-check.enabledPackages ++ [
              pkgs.cdrtools
              pkgs.isomd5sum
              pkgs.p7zip
            ];
          };
          formatter = fmt.config.build.wrapper;
          checks = {
            pre-commit-check = inputs.pre-commit-hooks.lib.${system}.run {
              src = ./.;
              hooks = {
                # keep-sorted start case=no
                check-executables-have-shebangs.enable = true;
                check-shebang-scripts-are-executable.enable = true;
                detect-private-keys.enable = true;
                end-of-file-fixer.enable = true;
                nixfmt-rfc-style.enable = true;
                trim-trailing-whitespace.enable = true;
                # keep-sorted end
                end-of-file-fixer.excludes = [
                  ".cz.json"
                ];
              };
            };
          };
        }
      );
}
