# Changelog

## [2.1.0](https://github.com/TheMeinerLP/proxmox-license-proxy/compare/v2.0.0...v2.1.0) (2026-06-29)


### Features

* **client:** reuse saved proxy on enroll, repair pre-2.0 upgrade, OS-package completion ([#16](https://github.com/TheMeinerLP/proxmox-license-proxy/issues/16)) ([fcb3bc2](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/fcb3bc2a6fe91e5a968de130f38002363989cc45))


### Bug Fixes

* **install:** non-interactive deb install to survive conffile prompts ([#14](https://github.com/TheMeinerLP/proxmox-license-proxy/issues/14)) ([cebcd3d](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/cebcd3d3b9385fa4e5644c82ced36c493505a051))

## [2.0.0](https://github.com/TheMeinerLP/proxmox-license-proxy/compare/v1.3.0...v2.0.0) (2026-06-29)


### ⚠ BREAKING CHANGES

* add ACME-style v1 API to issue & manage subscriptions; move state to /etc/pmox ([#13](https://github.com/TheMeinerLP/proxmox-license-proxy/issues/13))
* **config:** the default registry_file is now /etc/pmox/registry.json. The package migrates existing data automatically; manual deployments that relied on the /var/lib/pmox default should set registry_file or move the file.

### Features

* add ACME-style v1 API to issue & manage subscriptions; move state to /etc/pmox ([#13](https://github.com/TheMeinerLP/proxmox-license-proxy/issues/13)) ([f1e50aa](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/f1e50aa3bf074f4c00597881eb763c8c95424f41))
* **cli:** guided menu for bare `subscription` and `client` ([62e58d9](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/62e58d9872403ce40f3e6b7f76a9663cbac6c8a4))
* **config:** keep registry and auto cert under /etc/pmox ([e37b473](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/e37b473fe43c1d35ccb2922885c664b2e4466e45))


### Bug Fixes

* **cli:** create config dir with 0750 (gosec G301) ([d22df3b](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/d22df3bc299803af80f30be31d1a530f8ac910a5))
* **cli:** default config init/setup output to /etc/pmox/config.yaml ([21ae6c7](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/21ae6c7263a5848a5863c6983e0119109dbba8c1))

## [1.3.0](https://github.com/TheMeinerLP/proxmox-license-proxy/compare/v1.2.0...v1.3.0) (2026-06-29)


### Features

* **cli:** check GitHub for newer releases (version --check, doctor) ([5d14729](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/5d14729d1302b94309af3b0a64aa538727b7c7a2))
* **cli:** guide subscription, server and config commands interactively ([3b05afb](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/3b05afbc294aa9dcf719b6ea21f4d967ecb246f7))
* **cli:** serve summary, doctor command, actionable empty states, guided cert/hosts ([59a1955](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/59a19556b59a077d30eddeadddca3e9a97f49f12))
* **completion:** annotate key and host completions with status ([0721bee](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/0721bee1ea6ee054a83c010852be7a8c0c97b1aa))


### Bug Fixes

* **deps:** update module golang.org/x/mod to v0.37.0 ([#11](https://github.com/TheMeinerLP/proxmox-license-proxy/issues/11)) ([552682c](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/552682c708955c50bc02db75445df920a0f7f731))
* **install:** report version from /usr/bin and warn on PATH shadowing ([41891aa](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/41891aac8849e32d771bcfd18035ac235da24929))
* **serve:** persist the auto TLS certificate across restarts ([448d071](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/448d071080931e55ee4af1d60eb44409821c1e48))


### Code Refactoring

* **cli:** rename license to subscription (Proxmox terminology) ([71ed028](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/71ed0288e0c94d44fdd85e6e527a7cfa0c5dd273))
* reduce duplication and harden the release/version comparison ([255ea27](https://github.com/TheMeinerLP/proxmox-license-proxy/commit/255ea27e63501a5c1d813d36a70a32f0b5bc7193))

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
