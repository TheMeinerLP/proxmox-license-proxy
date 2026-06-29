# Changelog

## [1.2.0](https://github.com/TheMeinerLP/proxmox-license-proxy/compare/v1.1.0...v1.2.0) (2026-06-29)


### Features

* **license:** ask for PVE CPU socket count when generating ([dcff1a9](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/dcff1a9b33be0a72fde2b9e40525e4a7f626cfc8))

## [1.1.0](https://github.com/TheMeinerLP/proxmox-license-proxy/compare/v1.0.0...v1.1.0) (2026-06-29)


### Features

* **certs:** show CA SHA-256 fingerprint on trust bootstrap ([356c829](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/356c829ca5d2dd587237c215aee0c7aa01e7a93f))
* **discovery:** offer localhost and .local host when picking a server ([654f88b](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/654f88b4187bbd2227e187593d3a6b843da3b78a))
* **install:** add CLI-only mode (binary + completions, no service) ([57a5725](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/57a5725ff8e31c01418755bc5e89ea99d8b60baf))
* **license:** interactive product and level picker for generate ([ca64f4a](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/ca64f4a8f978b7835d5faa51176db7cee7e7f75c))
* **packaging:** ship shell completions in deb/rpm/apk and archives ([a4ce135](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/a4ce135edd714eabe1332b67afcca0f9758533d4))
* **server:** auto-approve hosts from trusted networks via config ([a770293](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/a770293fda25ad9a3e756fb4e88cc7bcd8fb5dec))


### Bug Fixes

* **config:** default registry to /var/lib/pmox so CLI and service agree ([c23ddca](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/c23ddca99e6e69c830996cdee0d148fc0bcc1b69))
* **packaging:** enable mDNS under hardening and share the registry dir ([2635d00](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/2635d00fa0cdf283b78dc96560db4766b461c026))
* **registry:** pin registry files to 0660 for shared root/pmox access ([8b256e2](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/8b256e266ac220d629ca262754979aaa22eded9d))
* **subscription:** match real Proxmox key format (PVE socket digit) ([ae468e5](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/ae468e5004cdb348b3d613c8b4dedd04fa6e4e1d))


### Code Refactoring

* **fileio:** route file reads through one cleaned helper, drop last gosec exclude ([6e5377c](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/6e5377c27f04196c2157f6f15bff0ffc29d05bb2))


### Build System & Hardening

* **lint:** drop blanket gosec excludes, justify findings inline ([7e6f0e7](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/7e6f0e7e563d751e26d5a4da76cfd2490219dc48))
* **lint:** drop unused nolint on hosts atomicWrite ([128be13](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/128be1352015705e5bf6c9ba365409301d65bfe9))
* **lint:** rely on filepath.Clean to satisfy gosec G304 ([c8449d6](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/c8449d6ecc1b15c7737f6fe4b26cf82d0dc0b9e1))


### Documentation

* document CLI-only install, completions, and lab-key examples ([2c59a11](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/2c59a111a7023426444e9afcdf1c90b694b352ca))

## 1.0.0 (2026-06-29)


### Features

* add one-line installer with arch and package-format detection ([df77115](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/df771156cdf3601e5f8e8c13e5f48d1f023362db))
* **app:** add application service layer ([defb701](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/defb70195811267061a94e08fbe9d7404515bd31))
* **certs:** add self-signed certificate and trust helpers ([bfa802c](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/bfa802cb32905552448106797442625d37dcc794))
* **cli:** add command-line interface ([c6f6396](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/c6f639622c64e1bf385697696aae2e61222d99c9))
* **client:** add binary self-install and uninstall helpers ([deb8045](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/deb8045d6e9a70fc736f96f6537bbd03546fc37e))
* **config:** add layered configuration with validation ([c55a6b3](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/c55a6b3dbdf18dae674752f2c8b4834b096374c9))
* **discovery:** add mDNS server advertising and client browsing ([b666563](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/b66656317fb06abe47440ff6de43821f1a7cd2d5))
* **docker:** add container image and compose setup ([523accb](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/523accbed5ed804580c36f9022e43cf0a2ca1cb1))
* **hosts:** add /etc/hosts management ([7a50319](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/7a5031987afc43d2889e1d3e7b06b759e8a9982b))
* **httpapi:** add HTTP server with verify.php emulation, REST API and health probes ([85de207](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/85de2076a1898e3509f96b479fcae265c87aac46))
* **packaging:** add deb/rpm/apk packages with systemd unit ([e8d78ee](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/e8d78ee095a2085b59d76bdc4069562ae89f0069))
* **registry:** add JSON registry store with file locking ([ebeb9b2](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/ebeb9b2e91954ffbb8f825a8ed00bcdf3b3315c5))
* **subscription:** add Proxmox subscription protocol core ([47ed8ad](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/47ed8ad3130101edc844a33ba11086b4853f671f))


### Bug Fixes

* **deps:** bump golang.org/x/net and miekg/dns for darwin cross-compile ([a00929f](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/a00929f2c5d934dc3c2ecc6acc2e89b4ffe9a236))
