# Changelog

## [1.8.0](https://github.com/grikwong/crossagent/compare/v1.7.1...v1.8.0) (2026-04-24)


### Features

* **state:** harden revert operation atomicity ([bdf130c](https://github.com/grikwong/crossagent/commit/bdf130c6bae3d452e3e1f46d70753f0dac772b04))
* **web:** add collapsible terminal drawer to v2 layout ([b4412f1](https://github.com/grikwong/crossagent/commit/b4412f197b9fb5a10707508f5c39664ca253ed02))
* **web:** make pipeline-timeline the default layout ([94aa516](https://github.com/grikwong/crossagent/commit/94aa516ed969a3a8f13e63351508af632a1962cd))
* **web:** polish v2 — density, breakpoints, Raw, retries ([c8232a1](https://github.com/grikwong/crossagent/commit/c8232a1ee92cd0765e0bdd75fd866d9cd71e1610))
* **web:** render pipeline-timeline layout behind ?v2=1 flag ([2bafa5b](https://github.com/grikwong/crossagent/commit/2bafa5bf66adad69b24bfd90684916b55bfd183c))
* **web:** scaffold pipeline-timeline UI redesign ([1dd934b](https://github.com/grikwong/crossagent/commit/1dd934bd96ff5850cc54c21d013914210f8fbdfc))
* **web:** smaller font and repaint after fit ([1e719f4](https://github.com/grikwong/crossagent/commit/1e719f4ea3ca3bd640e91223a4092e521e4b73a3))


### Bug Fixes

* **web:** address pipeline-timeline review findings ([96136e7](https://github.com/grikwong/crossagent/commit/96136e76c4668f75182de5dc1be013a8fdd52b11))
* **web:** atomic spawn dedup and phase-aware advance guard ([db06ce7](https://github.com/grikwong/crossagent/commit/db06ce74746a22c99d40305df5c6a5bd7cbff25e))
* **web:** block desc edits during active session ([7120beb](https://github.com/grikwong/crossagent/commit/7120beb2ea716e7001ac66706d0a2d2955b4acb5))
* **web:** edge-trigger drawer fit and reorder rail ([61bb825](https://github.com/grikwong/crossagent/commit/61bb8252ee54ce41627425147a6d90d76561eb85))
* **web:** fix workflow selection revert bug ([94d966a](https://github.com/grikwong/crossagent/commit/94d966ae01c60085d43aa3f8764b48e8c92914da))
* **web:** search focus, alignment, description rail ([26359fa](https://github.com/grikwong/crossagent/commit/26359fa4ebb3b0d9891eefb29fa46e59902a7afc))

## [1.7.1](https://github.com/grikwong/crossagent/compare/v1.7.0...v1.7.1) (2026-04-16)


### Bug Fixes

* update Go to 1.25.9 to resolve GO-2026-4947 ([097fff8](https://github.com/grikwong/crossagent/commit/097fff83f022d819ee52a76f973f226f0764663b))

## [1.7.0](https://github.com/grikwong/crossagent/compare/v1.6.0...v1.7.0) (2026-04-16)


### Features

* add agents autoselect + Web UI model management ([a867c2c](https://github.com/grikwong/crossagent/commit/a867c2c5f6b1bb6e43c86e71269415dc4cd2aecf))
* add searchable workflow picker ([421c106](https://github.com/grikwong/crossagent/commit/421c106eef67d1c3eb66aac7892d125691893506))
* **agent:** add gemini adapter + sandbox-limited writes ([603de32](https://github.com/grikwong/crossagent/commit/603de3239ac7e748b211c6978a869214d9afaaec))
* current-workflow retry artifact viewer ([1fb897f](https://github.com/grikwong/crossagent/commit/1fb897ff6d7741929a253b91039470b86bdbf3e1))
* harden state and enhance sandbox recovery ([c6ab40c](https://github.com/grikwong/crossagent/commit/c6ab40c317a5ff1f8b08ee727cc3b96b83dadd8d))
* surface attempt artifacts in root-level status JSON ([4488328](https://github.com/grikwong/crossagent/commit/4488328bad56cd107ba0a415c5b9a3e8ffe7be76))
* **web:** 12-col grid layout + phase slots ([178e669](https://github.com/grikwong/crossagent/commit/178e669ecc665fbd644e565e38e4ef5c2afc7c86))
* **web:** yellow-boxed styling for sidebar icon buttons ([71707f6](https://github.com/grikwong/crossagent/commit/71707f6b0281d44a9e5c96dca0351be489302273))


### Bug Fixes

* **state:** harden recovery, state writes, and paths ([644b752](https://github.com/grikwong/crossagent/commit/644b752a3f6075bbc2eb5bf9b05f81367906ccb9))

## [1.6.0](https://github.com/grikwong/crossagent/compare/v1.5.0...v1.6.0) (2026-04-06)


### Features

* add followup rounds for completed workflows ([fb12a84](https://github.com/grikwong/crossagent/commit/fb12a84c195fca1585eb912736013a2a4f1e0964))

## [1.5.0](https://github.com/grikwong/crossagent/compare/v1.4.0...v1.5.0) (2026-03-27)


### Features

* responsive web UI and mobile sidebar ([e3fc15b](https://github.com/grikwong/crossagent/commit/e3fc15bbbdb4cfaa2a4c8d1e9b31bfff600f0ba3))
* upgrade xterm.js to 6.x ([d3bee77](https://github.com/grikwong/crossagent/commit/d3bee77368280dbe6daa1fe7b8523f88d0485f9d))


### Bug Fixes

* fix macOS sandbox permission issues by resolving symlinks and authorizing home directory ([6f7a8b9](https://github.com/grikwong/crossagent/commit/6f7a8b9))
* manual Mode 2026 synchronized output ([f3f7600](https://github.com/grikwong/crossagent/commit/f3f7600ccbc899cc693cf38d18a4add32a64e150))

## [1.4.0](https://github.com/grikwong/crossagent/compare/v1.3.0...v1.4.0) (2026-03-24)


### Features

* add commit-aware version display for dev builds ([baad60f](https://github.com/grikwong/crossagent/commit/baad60fb4d234960fb134b8558219b8586b984ae))
* multi-session isolation and retry fix ([e71ea65](https://github.com/grikwong/crossagent/commit/e71ea6576515921d534d9039efa3c25f3c6ecb93))


### Bug Fixes

* remove legacy bash parity test, fix ws API ([69f8c43](https://github.com/grikwong/crossagent/commit/69f8c43a0502c014dbe70bf414ac72c0db7713c6))
* validate /api/version in web test ([b574459](https://github.com/grikwong/crossagent/commit/b5744591c889b6655b1c503973b5dbcb03cff5f4))

## [1.3.0](https://github.com/grikwong/crossagent/compare/v1.2.0...v1.3.0) (2026-03-18)


### Features

* add auto-install preflight for missing dependencies ([b933dea](https://github.com/grikwong/crossagent/commit/b933dea44be447d4292e066921334c11dbde6c01))
* add branding assets and favicon ([84b8730](https://github.com/grikwong/crossagent/commit/84b87309183d332e28a67fa955e284b036d0fea2))
* add chat history capture for PTY ([9191b93](https://github.com/grikwong/crossagent/commit/9191b93fcbca36db56283a8d4e072839ae13c23b))
* add Go web server replacing Node.js ([f42b9db](https://github.com/grikwong/crossagent/commit/f42b9db12070577944d7eecfdba9c5078265ba99))
* add internal/web embed and CreateWorkflow ([3fde74f](https://github.com/grikwong/crossagent/commit/3fde74fd7eda9f4db75554da79a2667c5ee9ae24))
* add project hierarchy with CLI management commands ([b075b37](https://github.com/grikwong/crossagent/commit/b075b3780bb18d6786d83bd78d1c8e5b36dadcd6))
* CrossAgent multi-model AI workflow orchestrator ([ee4e9c6](https://github.com/grikwong/crossagent/commit/ee4e9c6cb5d6afbb9f38a4c3bbd11f52c635a4f5))
* finalize Go migration and docs ([f7edc17](https://github.com/grikwong/crossagent/commit/f7edc177b96a5700b4bc4dbb223186c5da2ec648))
* implement project hierarchy + scoped memory ([ee57231](https://github.com/grikwong/crossagent/commit/ee57231d79d59e07d9b050af427089c6f3548384))
* Phase 1 Go rewrite — data layer & state management ([0594634](https://github.com/grikwong/crossagent/commit/0594634824bd62b991b272f29b81bc9982a0d022))
* Phase 2 Go — agents, prompts & judging ([da4cd7e](https://github.com/grikwong/crossagent/commit/da4cd7ef23f9509c305226a813a523d4d29c536b))
* Phase 3A Go — CLI command dispatch ([ed6acdc](https://github.com/grikwong/crossagent/commit/ed6acdc6c2e28a47e1350817d57104e074135ce5))
* Phase 3B Go — JSON output parity ([0f80d56](https://github.com/grikwong/crossagent/commit/0f80d56efdf74e95450e2c0078565ac2e9816542))
* update README to incorporate the feature highlights ([3f635ea](https://github.com/grikwong/crossagent/commit/3f635eac69aa09139677cc1827209b60f095fec7))


### Bug Fixes

* add chat_history to bash legacy JSON ([2f94473](https://github.com/grikwong/crossagent/commit/2f944737c024771eb0cf4a44a0c5eb23c5f1111a))
* add monorepo tag_prefix for goreleaser ([6bf449f](https://github.com/grikwong/crossagent/commit/6bf449f211327e66fe22eb144eb766df4dd24f76))
* drop component prefix from release tags ([0e123d7](https://github.com/grikwong/crossagent/commit/0e123d77810448854e5ca918fd43b5bfa2e65478))
* gate preflight tests for CI compat ([c7ea456](https://github.com/grikwong/crossagent/commit/c7ea4563c64290fabd7e062465a1aecab896fe69))
* match capture flag in removeEventListener ([c79b8c3](https://github.com/grikwong/crossagent/commit/c79b8c3a2a884c3a11c0588ff7f559303666c19c))
* upgrade Go minimum version from 1.22 to 1.25 ([ef870d4](https://github.com/grikwong/crossagent/commit/ef870d4e87674be182d7ae8b985bada2746542e5))
* use brews section in goreleaser ([2811222](https://github.com/grikwong/crossagent/commit/28112220cabfadd906956ea690ca00d36b0c61d3))
* web PTY rendering and tour overlay layering ([20dfff2](https://github.com/grikwong/crossagent/commit/20dfff2a5744041ca8b6d51d5385b2a87ccf8e3f))

## [1.2.0](https://github.com/grikwong/crossagent/compare/v1.1.0...v1.2.0) (2026-03-18)


### Features

* add auto-install preflight for missing dependencies ([b933dea](https://github.com/grikwong/crossagent/commit/b933dea44be447d4292e066921334c11dbde6c01))
* add branding assets and favicon ([84b8730](https://github.com/grikwong/crossagent/commit/84b87309183d332e28a67fa955e284b036d0fea2))
* add chat history capture for PTY ([9191b93](https://github.com/grikwong/crossagent/commit/9191b93fcbca36db56283a8d4e072839ae13c23b))
* add Go web server replacing Node.js ([f42b9db](https://github.com/grikwong/crossagent/commit/f42b9db12070577944d7eecfdba9c5078265ba99))
* add internal/web embed and CreateWorkflow ([3fde74f](https://github.com/grikwong/crossagent/commit/3fde74fd7eda9f4db75554da79a2667c5ee9ae24))
* add project hierarchy with CLI management commands ([b075b37](https://github.com/grikwong/crossagent/commit/b075b3780bb18d6786d83bd78d1c8e5b36dadcd6))
* CrossAgent multi-model AI workflow orchestrator ([ee4e9c6](https://github.com/grikwong/crossagent/commit/ee4e9c6cb5d6afbb9f38a4c3bbd11f52c635a4f5))
* finalize Go migration and docs ([f7edc17](https://github.com/grikwong/crossagent/commit/f7edc177b96a5700b4bc4dbb223186c5da2ec648))
* implement project hierarchy + scoped memory ([ee57231](https://github.com/grikwong/crossagent/commit/ee57231d79d59e07d9b050af427089c6f3548384))
* Phase 1 Go rewrite — data layer & state management ([0594634](https://github.com/grikwong/crossagent/commit/0594634824bd62b991b272f29b81bc9982a0d022))
* Phase 2 Go — agents, prompts & judging ([da4cd7e](https://github.com/grikwong/crossagent/commit/da4cd7ef23f9509c305226a813a523d4d29c536b))
* Phase 3A Go — CLI command dispatch ([ed6acdc](https://github.com/grikwong/crossagent/commit/ed6acdc6c2e28a47e1350817d57104e074135ce5))
* Phase 3B Go — JSON output parity ([0f80d56](https://github.com/grikwong/crossagent/commit/0f80d56efdf74e95450e2c0078565ac2e9816542))
* update README to incorporate the feature highlights ([3f635ea](https://github.com/grikwong/crossagent/commit/3f635eac69aa09139677cc1827209b60f095fec7))


### Bug Fixes

* add chat_history to bash legacy JSON ([2f94473](https://github.com/grikwong/crossagent/commit/2f944737c024771eb0cf4a44a0c5eb23c5f1111a))
* add monorepo tag_prefix for goreleaser ([6bf449f](https://github.com/grikwong/crossagent/commit/6bf449f211327e66fe22eb144eb766df4dd24f76))
* drop component prefix from release tags ([0e123d7](https://github.com/grikwong/crossagent/commit/0e123d77810448854e5ca918fd43b5bfa2e65478))
* gate preflight tests for CI compat ([c7ea456](https://github.com/grikwong/crossagent/commit/c7ea4563c64290fabd7e062465a1aecab896fe69))
* match capture flag in removeEventListener ([c79b8c3](https://github.com/grikwong/crossagent/commit/c79b8c3a2a884c3a11c0588ff7f559303666c19c))
* upgrade Go minimum version from 1.22 to 1.25 ([ef870d4](https://github.com/grikwong/crossagent/commit/ef870d4e87674be182d7ae8b985bada2746542e5))
* use brews section in goreleaser ([2811222](https://github.com/grikwong/crossagent/commit/28112220cabfadd906956ea690ca00d36b0c61d3))
* web PTY rendering and tour overlay layering ([20dfff2](https://github.com/grikwong/crossagent/commit/20dfff2a5744041ca8b6d51d5385b2a87ccf8e3f))

## [1.1.0](https://github.com/grikwong/crossagent/compare/crossagent-v1.0.0...crossagent-v1.1.0) (2026-03-18)


### Features

* add auto-install preflight for missing dependencies ([b933dea](https://github.com/grikwong/crossagent/commit/b933dea44be447d4292e066921334c11dbde6c01))
* add branding assets and favicon ([84b8730](https://github.com/grikwong/crossagent/commit/84b87309183d332e28a67fa955e284b036d0fea2))
* add chat history capture for PTY ([9191b93](https://github.com/grikwong/crossagent/commit/9191b93fcbca36db56283a8d4e072839ae13c23b))
* add Go web server replacing Node.js ([f42b9db](https://github.com/grikwong/crossagent/commit/f42b9db12070577944d7eecfdba9c5078265ba99))
* add internal/web embed and CreateWorkflow ([3fde74f](https://github.com/grikwong/crossagent/commit/3fde74fd7eda9f4db75554da79a2667c5ee9ae24))
* add project hierarchy with CLI management commands ([b075b37](https://github.com/grikwong/crossagent/commit/b075b3780bb18d6786d83bd78d1c8e5b36dadcd6))
* CrossAgent multi-model AI workflow orchestrator ([ee4e9c6](https://github.com/grikwong/crossagent/commit/ee4e9c6cb5d6afbb9f38a4c3bbd11f52c635a4f5))
* finalize Go migration and docs ([f7edc17](https://github.com/grikwong/crossagent/commit/f7edc177b96a5700b4bc4dbb223186c5da2ec648))
* implement project hierarchy + scoped memory ([ee57231](https://github.com/grikwong/crossagent/commit/ee57231d79d59e07d9b050af427089c6f3548384))
* Phase 1 Go rewrite — data layer & state management ([0594634](https://github.com/grikwong/crossagent/commit/0594634824bd62b991b272f29b81bc9982a0d022))
* Phase 2 Go — agents, prompts & judging ([da4cd7e](https://github.com/grikwong/crossagent/commit/da4cd7ef23f9509c305226a813a523d4d29c536b))
* Phase 3A Go — CLI command dispatch ([ed6acdc](https://github.com/grikwong/crossagent/commit/ed6acdc6c2e28a47e1350817d57104e074135ce5))
* Phase 3B Go — JSON output parity ([0f80d56](https://github.com/grikwong/crossagent/commit/0f80d56efdf74e95450e2c0078565ac2e9816542))
* update README to incorporate the feature highlights ([3f635ea](https://github.com/grikwong/crossagent/commit/3f635eac69aa09139677cc1827209b60f095fec7))


### Bug Fixes

* add chat_history to bash legacy JSON ([2f94473](https://github.com/grikwong/crossagent/commit/2f944737c024771eb0cf4a44a0c5eb23c5f1111a))
* gate preflight tests for CI compat ([c7ea456](https://github.com/grikwong/crossagent/commit/c7ea4563c64290fabd7e062465a1aecab896fe69))
* match capture flag in removeEventListener ([c79b8c3](https://github.com/grikwong/crossagent/commit/c79b8c3a2a884c3a11c0588ff7f559303666c19c))
* upgrade Go minimum version from 1.22 to 1.25 ([ef870d4](https://github.com/grikwong/crossagent/commit/ef870d4e87674be182d7ae8b985bada2746542e5))
* use brews section in goreleaser ([2811222](https://github.com/grikwong/crossagent/commit/28112220cabfadd906956ea690ca00d36b0c61d3))
* web PTY rendering and tour overlay layering ([20dfff2](https://github.com/grikwong/crossagent/commit/20dfff2a5744041ca8b6d51d5385b2a87ccf8e3f))
