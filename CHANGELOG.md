# Changelog

All notable changes to the Wave-Engineering fork of [SwarmCD](https://github.com/m-adawi/swarm-cd) are documented here.

This fork diverges from upstream at **v1.10.0**. For upstream history prior to that, see the [original repository](https://github.com/m-adawi/swarm-cd).

## [1.17.0](https://github.com/Wave-Engineering/swarm-cd/compare/v1.16.0...v1.17.0) (2026-03-22)

### Features

* **ui:** migrate to snake_case API, add timestamps, health footer, warning banner ([#21](https://github.com/Wave-Engineering/swarm-cd/issues/21)), closes [#13](https://github.com/Wave-Engineering/swarm-cd/issues/13)
  - All UI components updated to snake_case field names matching the new API response format
  - New `useFetchHealth` hook polls `GET /health` every 30s
  - New `HealthFooter` component displays version, uptime, boot time, and stack count
  - New `WarningBanner` component shows dismissible config warnings
  - Relative timestamp display for last changed/deployed times
  - Null-safe filtering in `StatusCardList`

## [1.16.0](https://github.com/Wave-Engineering/swarm-cd/compare/v1.15.0...v1.16.0) (2026-03-22)

### Features

* **util:** add config conflict detection and warnings ([#20](https://github.com/Wave-Engineering/swarm-cd/issues/20)), closes [#9](https://github.com/Wave-Engineering/swarm-cd/issues/9)
  - Detects when inline `config.yaml` shadows split config files (`stacks.yaml`/`repos.yaml`)
  - Warnings surfaced via `GET /health` response (`config_warnings` field)

## [1.15.0](https://github.com/Wave-Engineering/swarm-cd/compare/v1.14.0...v1.15.0) (2026-03-22)

### Features

* **swarmcd:** add config persistence and `PATCH /stacks/{name}` endpoint ([#19](https://github.com/Wave-Engineering/swarm-cd/issues/19)), closes [#11](https://github.com/Wave-Engineering/swarm-cd/issues/11)
  - Runtime mutation of stack configuration (repo URL, branch/tag, compose file) via PATCH
  - 3-phase locking pattern: validate (RLock) → I/O (no locks) → apply (Lock)
  - Atomic YAML persistence with write-to-temp-then-rename
  - Clone-to-temp-then-swap for safe repo URL changes
  - Typed error responses (`ValidationError` → 400, `NotFoundError` → 404)

## [1.14.0](https://github.com/Wave-Engineering/swarm-cd/compare/v1.13.0...v1.14.0) (2026-03-22)

### Features

* **web:** add `GET /stacks/{name}` and enhance `GET /health` endpoint ([#18](https://github.com/Wave-Engineering/swarm-cd/issues/18)), closes [#8](https://github.com/Wave-Engineering/swarm-cd/issues/8)
  - Individual stack lookup by name
  - Health endpoint returns version, uptime, boot time, stack count, update interval, mutation API status

## [1.13.0](https://github.com/Wave-Engineering/swarm-cd/compare/v1.12.0...v1.13.0) (2026-03-22)

### Features

* **web:** add restart endpoints via Docker Engine API ([#17](https://github.com/Wave-Engineering/swarm-cd/issues/17)), closes [#12](https://github.com/Wave-Engineering/swarm-cd/issues/12)
  - `POST /stacks/{name}/restart` — restart all services in a stack
  - `POST /stacks/{name}/services/{service}/restart` — restart a single service
  - `POST /restart` — restart all managed services
  - Uses Docker `ForceUpdate` counter increment for zero-downtime restarts

## [1.12.0](https://github.com/Wave-Engineering/swarm-cd/compare/v1.11.0...v1.12.0) (2026-03-22)

### Features

* **swarmcd:** enhance StackStatus data model and `GET /stacks` response ([#16](https://github.com/Wave-Engineering/swarm-cd/issues/16)), closes [#7](https://github.com/Wave-Engineering/swarm-cd/issues/7)
  - Snake_case JSON field names in API responses
  - New fields: `ref_type`, `ref_value`, `compose_file`, `last_change_at`, `last_deployed_at`
  - Replaces the combined `Ref` field with separate type/value pair

## [1.11.0](https://github.com/Wave-Engineering/swarm-cd/compare/v1.10.1...v1.11.0) (2026-03-22)

### Features

* **swarmcd:** add `sync.RWMutex` and lock ordering for concurrency safety ([#14](https://github.com/Wave-Engineering/swarm-cd/issues/14)), closes [#6](https://github.com/Wave-Engineering/swarm-cd/issues/6)
  - Protects shared state from concurrent access between reconciler and API handlers
  - Enforces lock ordering: `stateMu` must never be acquired while `repo.lock` is held
* **web:** add bearer token auth middleware for write endpoints ([#15](https://github.com/Wave-Engineering/swarm-cd/issues/15)), closes [#10](https://github.com/Wave-Engineering/swarm-cd/issues/10)
  - `SWARMCD_API_TOKEN` environment variable enables the mutation API
  - Timing-safe token comparison via `crypto/subtle.ConstantTimeCompare`

## [1.10.1](https://github.com/Wave-Engineering/swarm-cd/compare/v1.10.0...v1.10.1) (2026-02-16)

### Bug Fixes

* remove Docker Hub from CI workflow ([#2](https://github.com/Wave-Engineering/swarm-cd/issues/2))
  - Fork CI no longer requires Docker Hub credentials

### Other

* add git tag support and display watched ref in UI ([#1](https://github.com/Wave-Engineering/swarm-cd/issues/1))
  - Stacks can watch git tags in addition to branches
  - UI displays the watched ref type and value
