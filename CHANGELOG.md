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
