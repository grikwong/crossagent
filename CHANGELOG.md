# Changelog

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
