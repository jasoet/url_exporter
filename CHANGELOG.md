# [2.2.0](https://github.com/jasoet/url_exporter/compare/v2.1.2...v2.2.0) (2025-07-29)


### Bug Fixes

* resolve test failures for TLS certificate and URL handling ([dec5a0b](https://github.com/jasoet/url_exporter/commit/dec5a0bb6914cc4b7b2b899b4d09c0a8b90b0a0c))


### Features

* add multi-protocol support beyond HTTP/HTTPS ([baa36b1](https://github.com/jasoet/url_exporter/commit/baa36b1932b129a14a1a32aa119ef13f2639d083))

## [2.1.2](https://github.com/jasoet/url_exporter/compare/v2.1.1...v2.1.2) (2025-07-27)


### Bug Fixes

* revert Go version back to 1.24.5 and prepare to fix go.sum ([e19fc3a](https://github.com/jasoet/url_exporter/commit/e19fc3abdb5c2d98bb40b46a1de4295ad2b5bdc7))
* update Go version from 1.24.5 to 1.23 to resolve build failure ([7271f79](https://github.com/jasoet/url_exporter/commit/7271f79ec50369c3ab08619eee90c6e00fd18cda))
* update test assertion for context cancellation ([6a85f1b](https://github.com/jasoet/url_exporter/commit/6a85f1b6efaa2976659d76e516318fec07228b23))
* update test assertions to match current error handling behavior ([4e6f01d](https://github.com/jasoet/url_exporter/commit/4e6f01d9901ca448d258beacbccfe4129933d96b))

## [2.1.1](https://github.com/jasoet/url_exporter/compare/v2.1.0...v2.1.1) (2025-07-27)


### Bug Fixes

* resolve test failures and wrapped error handling issues ([5b6a2ff](https://github.com/jasoet/url_exporter/commit/5b6a2ff286cbebb92614afefce420cde279f05ed))

# [2.1.0](https://github.com/jasoet/url_exporter/compare/v2.0.0...v2.1.0) (2025-07-25)


### Features

* update build metadata to reflect personal attribution ([23956e0](https://github.com/jasoet/url_exporter/commit/23956e058985a2fceb4fbf5f43d173127c8c5960))

# [2.0.0](https://github.com/jasoet/url_exporter/compare/v1.0.0...v2.0.0) (2025-07-25)


### Features

* implement comprehensive metrics and version integration ([3182f94](https://github.com/jasoet/url_exporter/commit/3182f94afd309af120f4404457599dcf2a741779))


### BREAKING CHANGES

* Server constructor now requires version information parameter

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>

# 1.0.0 (2025-07-25)


### Bug Fixes

* **goreleaser:** update default config file name in release artifacts ([e76d49b](https://github.com/jasoet/url_exporter/commit/e76d49bf3720a6e0d6092225ef31b542681f3428))


### Features

* add url_error metric to distinguish network errors from HTTP errors ([eb787af](https://github.com/jasoet/url_exporter/commit/eb787af810edc2e57ac7d144b3f6c972eebab722))
* improve metrics precision and simplify configuration ([e93205e](https://github.com/jasoet/url_exporter/commit/e93205eaf38aef499e36921ae7382d07c66a0ebb))


### BREAKING CHANGES

* Response time metric unit changed from seconds to milliseconds.
Existing Prometheus queries and dashboards need to be updated.
