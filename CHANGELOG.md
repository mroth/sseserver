# Changelog

All notable changes to this project should be documented in this file.

## [1.1.0] - 2018-07-20

- `sseserver.Server` now explicitly implements `http.Handler`, allowing it to
  more easily be integrated into existing HTTP applications.
- internal cleanup: modularize internals, may lead to exposing more in future
  release
- reduction of memory allocations in core message formatting loop
- implement keepalive pings
- allow admin endpoint to be disabled in settings

## 1.0.0 - 2014-07-29

- Initial release

[1.1.0]: https://github.com/mroth/sseserver/compare/v1.0.0...v1.1.0
