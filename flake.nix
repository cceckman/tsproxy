{
  description = "A Tailscale reverse-proxy";

  # Nixpkgs / NixOS version to use.
  inputs.nixpkgs.url = "nixpkgs/nixos-22.11";
  inputs.utils.url = "github:numtide/flake-utils";

  outputs = { self, nixpkgs, utils }:
  utils.lib.eachDefaultSystem (system:
    let
      pkgs = import nixpkgs { inherit system; };
    in
    {
      packages = rec {
        tsproxy = pkgs.buildGoModule {
          name = "tsproxy";
          src = ./.;
          runVend = true;
          # proxyVend = true;
          vendorSha256 = "sha256-HSYkoEXdOPKzZ2P+YQqg8+cxSPUcQnPUQCguMbrVBjw=";
        };
        default = tsproxy;
      };

      devShells = {
        default = pkgs.mkShell {
          buildInputs = with pkgs; [ go gopls gotools go-tools ];
        };
      };

      # NixOS module; use submodules to configure instances:
      # https://nixos.org/manual/nixos/stable/#section-option-types-submodule
      nixosModules.default = { config, lib, ... } :
      let
        instance-config = lib.types.submodule {
          options = {
            hostname = lib.mkOption {
              type = lib.types.str;
              description = "Tailscale node name to register this proxy as";
            };
            target = lib.mkOption {
              type = lib.types.str;
              description = "Address to proxy to";
            };
            authKeyPath = lib.mkOption {
              type = lib.types.str;
              description = "Filesystem path to read a Tailscale auth key from";
              default = "";
            };
          };
        };
        instantiate = {hostname, target, authKeyPath}: {
          "tsproxy-${hostname}" = let
            authKeyFlag = if authKeyPath == "" then "" else "-authKeyPath ${authKeyPath}";
          in {
            description = "Tailscale proxy from ${hostname} to ${target}";
            path = [ "${self.packages."${system}".default}" ];
            script = ''
            tsproxy -from ${hostname} -to ${target} ${authKeyFlag}
            '';
            wantedBy = ["multi-user.target"];
            after = ["network-online.target"];
          };
        };
      in {
        options = {
          services.tsproxy.instances = lib.mkOption {
            type = lib.types.listOf instance-config;
            description = "Instances of tsproxy to run.";
            default = [];
          };
        };
        config = lib.mkIf (config.services.tsproxy.instances != []) {
          systemd.services = builtins.foldl' (x: y: (x // y)) {} (
            builtins.map instantiate config.services.tsproxy.instances
          );
        };
      };
    }
  );
}
