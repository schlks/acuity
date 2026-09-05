{
  description = "Acuity - Photo Management Service";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system;
        };

        frontend = pkgs.buildNpmPackage {
          pname = "acuity-frontend";
          version = "0.1.0";
          src = ./ui;

          npmDepsHash = "sha256-Zpl2p4KaF9Vx8LpyCBswlHJx/GfthdjPOSu1H89nL8M=";
          npmFlags = [ "--ignore-scripts" ];
          makeCacheWritable = true;

          installPhase = ''
            						mkdir -p $out
            						cp -r build/* $out/
            					'';
        };

        acuity = pkgs.buildGoModule rec {
          pname = "acuity";
          version = "0.1.0";
          src = ./.;

          vendorHash = "sha256-NW07rOY2yvZxFKmXeU0HAWKXhXC5ZanImmp6qZWKwek=";

          doCheck = false;
          subPackages = [ "." ];

          env.CGO_ENABLED = 1;
          tags = [ "production" "webkit2_41" ];
          ldflags = [ "-s" "-w" ];

          buildInputs = with pkgs; [
            webkitgtk_4_1
            gtk3
            vips
            sqlite
            glib
            glib-networking
            gsettings-desktop-schemas
            adwaita-icon-theme
            librsvg
            cairo
            pango
            gdk-pixbuf
            at-spi2-core
            libsoup_3
          ];

          nativeBuildInputs = with pkgs; [
            pkg-config
            wrapGAppsHook3
          ];

          preBuild = ''
            						mkdir -p ui/build
            						cp -r ${frontend}/* ui/build/
            					'';

          postInstall = ''
            						install -Dm644 ui/static/logo.svg $out/share/icons/hicolor/scalable/apps/acuity.svg
            						mkdir -p $out/share/applications
            						cat > $out/share/applications/acuity.desktop <<EOF
            [Desktop Entry]
            Name=Acuity
            Comment=Photo Culling & Management App
            Exec=$out/bin/acuity %U
            Icon=acuity
            Terminal=false
            Type=Application
            Categories=Graphics;Photography;
            EOF
            					'';

          meta = with pkgs.lib; {
            description = "Photo Management Tool";
            homepage = "https://codeberg.org/Shlks/acuity";
            license = licenses.gpl3Only;
            mainProgram = "acuity";
          };
        };
        # nixpkgs' `wails` package wraps its binary with a hardcoded
        # PKG_CONFIG_PATH (for wails' own webview build deps), which
        # *replaces* rather than extends whatever PKG_CONFIG_PATH the
        # devShell set up. That breaks cgo deps of the app itself (vips
        # via bimg, sqlite via sqlite-vec) whenever `wails dev`/`wails
        # build` shells out to `go`. `nix develop -c wails ...` execs the
        # binary directly (no shell function resolution), so shadow it
        # with a real executable that calls the unwrapped binary instead.
        wailsUnwrapped = pkgs.writeShellScriptBin "wails" ''
          exec "${pkgs.wails}/bin/.wails-wrapped" "$@"
        '';
      in
      {
        packages.default = acuity;
        packages.acuity = acuity;

        devShells.default = pkgs.mkShell {
          nativeBuildInputs = with pkgs; [
            pkg-config
            go
            bun
            wailsUnwrapped
          ];

          buildInputs = acuity.buildInputs;

          shellHook = ''
            						export CGO_ENABLED=1
            						export PKG_CONFIG_PATH="${pkgs.lib.makeSearchPathOutput "dev" "lib/pkgconfig" acuity.buildInputs}:$PKG_CONFIG_PATH"
            					'';
        };
      }
    );
}
