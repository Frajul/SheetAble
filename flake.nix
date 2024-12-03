{
  description = "SheetAble";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-23.11";
    # nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        frontend = pkgs.writeShellScriptBin "frontend" ''
          cd frontend
          ${pkgs.nodejs_20}/bin/npm start
        '';
        backend = pkgs.writeShellScriptBin "backend" ''
          cd backend
          ${pkgs.go}/bin/go run .
        '';
      in
      rec {
        packages.backend = pkgs.buildGoModule {
          pname = "backend";
          version = "v0.8.1";
          src = ./backend;
          vendorHash = "sha256-6flC+9pTp6cZxFEA7CL704hpL6Ajh63Kg/Fq4XpfFdg=";
        };
        apps.frontend = {
          type = "app";
          program = "${frontend}/bin/frontend";
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            frontend
            backend

            go
            godef # development only
            gotools # developmet only
            gopls # development only
            go-rice
            nodejs_20
            nodePackages.serve

            pkg-config
            vips
          ];

          # shellHook = ''
          # '';

          # envvars
          # DEV=1;
        };

        packages.default = packages.backend;
      }
    );
}
