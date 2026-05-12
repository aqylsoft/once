# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-05-12

### Added

- Idempotency middleware for `net/http`
- `MemoryStore` with TTL and background cleanup
- Per-key locking to prevent thundering herd
- Request body hash validation (`WithRequestHashCheck`)
- Functional options: `WithHeader`, `WithTTL`, `WithRequireKey`, `WithCacheableStatus`
- C shared library (`libonce.so`) via cgo
- C API: `once_init`, `once_destroy`, `once_check`, `once_store`, `once_lock`, `once_unlock`

[Unreleased]: https://github.com/aqylsoft/once/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/aqylsoft/once/releases/tag/v0.1.0
