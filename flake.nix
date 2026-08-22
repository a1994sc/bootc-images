{
  description = "Nix packages";
  inputs = {
    # keep-sorted start block=yes case=no
    flake-utils = {
      inputs.systems.follows = "systems";
      url = "github:numtide/flake-utils";
    };
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
    # keep-sorted end
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
              programs.nixfmt.enable = true;
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
              pkgs.oras
              pkgs.rpm
            ];
          };
          formatter = fmt.config.build.wrapper;
          checks = {
            pre-commit-check = inputs.pre-commit-hooks.lib.${system}.run {
              src = ./.;
              hooks =
                let
                  excludes = [
                    ".cz.json"
                    "vendor/"
                  ];
                in
                {
                  # keep-sorted start case=no
                  check-executables-have-shebangs.enable = true;
                  check-executables-have-shebangs.excludes = excludes;
                  check-shebang-scripts-are-executable.enable = true;
                  check-shebang-scripts-are-executable.excludes = excludes;
                  detect-private-keys.enable = true;
                  detect-private-keys.excludes = excludes;
                  end-of-file-fixer.enable = true;
                  end-of-file-fixer.excludes = excludes;
                  nixfmt-rfc-style.enable = true;
                  nixfmt-rfc-style.excludes = excludes;
                  trim-trailing-whitespace.enable = true;
                  trim-trailing-whitespace.excludes = excludes;
                  # keep-sorted end
                };
            };
          };
        }
      );
}
