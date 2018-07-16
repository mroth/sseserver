# Changelog

All notable changes to this project should be documented in this file.

## [Unreleased]

- `sseserver.Server` now explicitly implements `http.Handler`, allowing it to more
  easily integrated into existing HTTP applications if desired.
- internal cleanup: modularize internals, may lead to exposing more in future release
- reduction of memory allocations in core message formatting loop
- implement keepalive pings
- allow admin endpoint to be disabled in settings

## 1.0.0 - 2014-07-29

- Initial release

[Unreleased]: https://github.com/mroth/sseserver/compare/v1.0.0...HEAD
