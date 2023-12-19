{
  description = "SheetAble";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-23.11";
    # nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem
      (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          run-frontend = pkgs.writeShellScriptBin "run-frontend" ''
            cd frontend
            ${pkgs.nodejs_20}/bin/npm start
          '';
        in rec {
          packages.backend = pkgs.buildGoModule {
            pname = "backend";
            version = "v0.8.1";
            src = ./backend;
            vendorHash = "sha256-/E9xRAjUWhq1/jABYg83QAK5hyOrWDoIOi4jZ3KaUOs=";
          };
          apps.frontend = {
            type = "app";
            program = "${run-frontend}/bin/run-frontend";
          };

          devShells.default = pkgs.mkShell {
            buildInputs = with pkgs; [
              run-frontend
              go
              nodejs_20
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
