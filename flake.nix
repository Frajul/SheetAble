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
        version = "0.9.0";
        pkgs = nixpkgs.legacyPackages.${system};
        frontend = pkgs.writeShellScriptBin "frontend" ''
          cd frontend
          ${pkgs.nodejs_20}/bin/npm start
        '';
        backend = pkgs.writeShellScriptBin "backend" ''
          cd backend
          ${pkgs.go}/bin/go run .
        '';
        build_frontend = pkgs.writeShellScriptBin "build_frontend" ''
          # Builds and embeds frontend into backend
          cd frontend
          ${pkgs.nodejs_20}/bin/npm install
          ${pkgs.nodejs_20}/bin/npm run build

          cd ../backend/api/controllers

          ${pkgs.go-rice}/bin/rice embed-go
        '';
        deploy = pkgs.writeShellScriptBin "deploy" ''
          set -e; set -o pipefail; set -x;

          nix build .#docker
          image=$((docker load < result) | sed -n '$s/^Loaded image: //p')
          ${pkgs.docker}/bin/docker image tag "$image" frajul/sheetable:latest
          ${pkgs.docker}/bin/docker push frajul/sheetable:latest
        '';
      in
      rec {
        packages.backend = pkgs.buildGoModule {
          pname = "backend";
          version = "v${version}";
          src = ./backend;
          vendorHash = "sha256-dP7ymyG+12YMHyoNrudeekj52iIrjtO8SC9VxE5unTs=";
          nativeBuildInputs = with pkgs; [
            pkg-config
          ];
          buildInputs = with pkgs; [
            vips
          ];
        };
        apps.frontend = {
          type = "app";
          program = "${frontend}/bin/frontend";
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            frontend
            backend
            build_frontend
            deploy

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

        };

        packages.default = packages.backend;

        packages.docker = pkgs.dockerTools.buildImage {
          name = "sheetable";
          tag = "latest";

          copyToRoot = with pkgs; [
            dockerTools.usrBinEnv
            dockerTools.binSh
            dockerTools.caCertificates

            coreutils
            packages.backend
          ];

          config = {
            Cmd = [ "/bin/backend" ];
            Expose = "8080";
          };
          created = "now";
        };

      }
    );
}
