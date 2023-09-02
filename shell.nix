{ pkgs ? import <nixpkgs> {} }:
    pkgs.mkShell {
        nativeBuildInputs = with pkgs.buildPackages; [
            glfw
            go
            pkg-config
            xorg.libX11.dev
            xorg.libXcursor
            xorg.libXft
            xorg.libXi
            xorg.libXinerama
            xorg.libXrandr
            xorg.libXxf86vm
        ];
}
