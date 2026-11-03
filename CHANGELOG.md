# Changelog

All notable changes to Bitswarm are listed here, newest first. The format follows [Keep a Changelog](https://keepachangelog.com), and versions follow [Semantic Versioning](https://semver.org). The version comes from the git tag.

## [1.0.0] - 2026-11-03

### Added
- Curl hooks: a third hook type that starts the real `curl` program with your command when an event fires. The editor has a template and there is a seeded example. The server Docker image includes curl.
- Paste curl in the hook editor: fills an API hook's method, URL, headers and body from a curl command, via `POST /api/hooks/parse-curl`. It never runs or sends anything.
- `{{secret.NAME}}` in API hooks, read from `SWARM_SECRET_NAME` or `SWARM_SECRET_NAME_FILE`, so keys never reach the database, logs, API or backups.
- Setting for the sidebar speed chart window (30 to 3600 seconds, default 90).
- CI for pull requests and pushes to `main`.
- Shareable URLs for the torrent list view, the settings section and the Execution log filters.

### Changed
- Replaced `react-router` with TanStack Router ([#9](https://github.com/bitswarm-dev/bitswarm/pull/9)): one file per route, a data loader per route, preloading on hover, a chunk per page, typed links, and shared pending, error and not-found screens. The torrent list view (filters, search, sort, selection, inspector tab), the settings section and the Execution log filters now live in the URL, so they can be bookmarked and shared. React Context holds only UI state.
- The README leads with automation and hooks.
- Stored header keys such as `Authorization` are masked after saving.
- Base images: golang 1.27, alpine 3.24, nginx 1.31, node 26. Build tools: Vite 6 and @vitejs/plugin-react 4.7.
- Third-party notices list the new dependencies and licenses.
- The hook editor no longer lists `SWARM_TORRENT_LABEL`, which was removed with labels.

### Security
- Fixed all `npm audit` findings, including an open-redirect XSS in the shipped router.
- Curl hooks only save when every placeholder follows a name and an equals sign (`name={{x}}`) or is `{{payload}}`, and `-F`/`--form` are refused. Values starting with `-`, `@` or `<` get a leading space. A torrent name can't make curl read a file or take an option.
- Database folder is `0700` and the file `0600`, tightened at startup.
- Query strings are removed from hook, schedule and update-feed errors.

### Fixed
- Command hooks that time out on Linux now stop the whole process group.
- The Execution log no longer re-renders in a loop.

Pull requests: [#1](https://github.com/bitswarm-dev/bitswarm/pull/1), [#2](https://github.com/bitswarm-dev/bitswarm/pull/2), [#3](https://github.com/bitswarm-dev/bitswarm/pull/3), [#4](https://github.com/bitswarm-dev/bitswarm/pull/4), [#5](https://github.com/bitswarm-dev/bitswarm/pull/5), [#6](https://github.com/bitswarm-dev/bitswarm/pull/6), [#7](https://github.com/bitswarm-dev/bitswarm/pull/7), [#8](https://github.com/bitswarm-dev/bitswarm/pull/8), [#9](https://github.com/bitswarm-dev/bitswarm/pull/9), [#10](https://github.com/bitswarm-dev/bitswarm/pull/10), [#11](https://github.com/bitswarm-dev/bitswarm/pull/11), [#13](https://github.com/bitswarm-dev/bitswarm/pull/13).

## [0.9.1] - 2026-11-03

### Added
- Release pipeline: a version tag creates the GitHub release and publishes the Docker images, binaries and dashboard build.

### Changed
- Releases are cut manually from a tag on `main`, with release branches only for older lines. The version comes from the tag.

## [0.9.0] - 2026-11-03

First release: a standalone BitTorrent client with a web interface, a built-in engine, scheduled downloads, rename presets, maintenance, hooks, API tokens, backups and an Execution log.

[1.0.0]: https://github.com/bitswarm-dev/bitswarm/compare/v0.9.1...v1.0.0
[0.9.1]: https://github.com/bitswarm-dev/bitswarm/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/bitswarm-dev/bitswarm/releases/tag/v0.9.0
