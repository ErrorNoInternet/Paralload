{
  description = "Paralload - A download tool that uses multiple HTTP(S) connections with byte ranges";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs = {
    flake-parts,
    nixpkgs,
    self,
    ...
  } @ inputs:
    flake-parts.lib.mkFlake {inherit inputs;} {
      systems = [
        "aarch64-linux"
        "x86_64-linux"
      ];
      perSystem = {
        system,
        pkgs,
        ...
      }: let
        nativeBuildInputs = with pkgs; [
          pkg-config
        ];
        buildInputs = with pkgs; [
          glfw
          xorg.libX11.dev
          xorg.libXcursor
          xorg.libXft
          xorg.libXi
          xorg.libXinerama
          xorg.libXrandr
          xorg.libXxf86vm
        ];
      in rec {
        devShells.default = pkgs.mkShell {
          name = "paralload";
          inherit nativeBuildInputs buildInputs;
        };

        packages.paralload = pkgs.buildGoModule {
          pname = "paralload";
          version =
            if (self ? shortRev)
            then self.shortRev
            else self.dirtyShortRev;

          src = pkgs.lib.cleanSource ./.;
          vendorHash = "sha256-q0RVXDdH8+cdCfY2PrMpE8lDlxo3lQgar9WtSjjwioc=";

          inherit nativeBuildInputs buildInputs;
        };
        packages.default = packages.paralload;
      };
    };
}
