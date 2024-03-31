{
  description = "Paralload - A download tool that uses multiple HTTP(S) connections with byte ranges";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs = {
    flake-parts,
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
      }:
        with pkgs; let
          nativeBuildInputs = [
            pkg-config
          ];
          buildInputs = [
            glfw
            xorg.libX11.dev
            xorg.libXcursor
            xorg.libXft
            xorg.libXi
            xorg.libXinerama
            xorg.libXrandr
            xorg.libXxf86vm
          ];
        in {
          devShells.default = mkShell {
            name = "paralload";
            inherit nativeBuildInputs buildInputs;
          };

          packages = rec {
            paralload = buildGoModule {
              pname = "paralload";
              version = self.shortRev or self.dirtyShortRev;

              src = lib.cleanSource ./.;
              vendorHash = "sha256-q0RVXDdH8+cdCfY2PrMpE8lDlxo3lQgar9WtSjjwioc=";

              inherit nativeBuildInputs buildInputs;
            };
            default = paralload;
          };
        };
    };
}
