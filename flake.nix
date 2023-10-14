{
  description = "Fast, multithreaded, modular and extensible DHCP server written in Go";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    gomod2nix.url = "github:nix-community/gomod2nix";
    gomod2nix.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = inputs@{ flake-parts, gomod2nix, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [ "x86_64-linux" "aarch64-linux" ];
      perSystem = { config, self', inputs', pkgs, system, ... }: {
        _module.args.pkgs = import inputs.nixpkgs {
          inherit system;
          overlays = [ gomod2nix.overlays.default ];
          config = { };
        };
        packages.default = pkgs.callPackage ./. { };
        devShells.default = import ./shell.nix { inherit pkgs; };
      };
    };
}
