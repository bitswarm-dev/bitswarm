<p align="center"><img src="docs/logo/bitswarm-logo.svg" alt="Bitswarm logo" width="96"></p>

# Bitswarm

[![Release](https://img.shields.io/github/v/release/bitswarm-dev/bitswarm)](https://github.com/bitswarm-dev/bitswarm/releases)
[![Docker pulls](https://img.shields.io/docker/pulls/dockerkeithics/bitswarm-server?label=docker%20pulls)](https://hub.docker.com/r/dockerkeithics/bitswarm-server)
[![Platforms](https://img.shields.io/badge/platforms-amd64%20%7C%20arm64%20%7C%20arm%2Fv7%20%7C%20386-blue)](https://hub.docker.com/r/dockerkeithics/bitswarm-server/tags)

A standalone BitTorrent client with a web interface. Bitswarm downloads and seeds by itself using a built-in engine, so you don't need qBittorrent, Transmission or any other client.

On top of the usual torrent list it adds automation: scheduled searches of websites, regex rename presets, maintenance tasks, hooks that call an API or run a command, an execution log, API tokens and settings backup.

> Bitswarm is a general-purpose BitTorrent client. It isn't built for piracy, and it doesn't provide, host or index content. Only download and share what you have the right to. You are responsible for following your local laws. This is not legal advice, and the authors aren't liable for how you use it.

## Contents

- [Screenshots](#screenshots)
- [Features](#features)
- [Install](#install)
- [Configuration](#configuration)
- [Not supported yet](#not-supported-yet)
- [Contributing](#contributing)
- [Security](#security)
- [License](#license)

## Screenshots

Captured in demo mode, which uses a simulated engine with sample torrents.

| Dark | Light |
|---|---|
| <img src="docs/screenshots/desktop-dark.png" alt="Bitswarm torrent list in the dark theme"> | <img src="docs/screenshots/desktop-light.png" alt="Bitswarm torrent list in the light theme"> |

<img src="docs/screenshots/scheduled-dark.png" alt="Bitswarm scheduled downloads page in the dark theme">

## Features

- **Torrents:** add by magnet link or `.torrent` file, pick which files to download, set per-file priorities, pause, resume, recheck, move files, rename on disk, queue, force start, global and alternative (scheduled) speed limits, seeding limits, restart recovery.
- **Tags:** group and filter torrents with tags. A torrent can have many.
- **Scheduled downloads:** search a site on a timer, read magnet or `.torrent` links with a CSS selector, filter by name, and add the new ones. Schedules can apply tags and a rename preset, and use the default save folder from Settings.
- **Auto rename:** regex presets (Go syntax) with a live preview, undo, and per-tag scope.
- **Maintenance:** six housekeeping tasks (retry errors, remove finished, handle stalled, low disk space, verify, clear old activity).
- **Hooks:** call an API (HMAC-signed, retried) or run a command on any event. Commands stay off unless you start the server with `SWARM_ALLOW_COMMANDS=1`.
- **API tokens:** Bearer tokens with read, control or admin access, under `/api/v1`.
- **Backup:** export and import settings, theme, tags, schedules, presets, maintenance tasks and hooks as JSON.
- **Interface:** light and dark themes, accent colors, background images, adjustable panel opacity, and a phone layout.

## Install

### Docker (recommended)

You only need [Docker](https://docs.docker.com/get-docker/) with Compose.

```sh
mkdir bitswarm && cd bitswarm
curl -fsSLO https://raw.githubusercontent.com/bitswarm-dev/bitswarm/main/docker-compose.yml
mkdir data downloads && chmod 777 data downloads
docker compose up -d
```

Then open <http://localhost:8080>. The default login is `admin` / `swarm`. **Change it right away** in Settings → Web UI.

The images [`dockerkeithics/bitswarm-server`](https://hub.docker.com/r/dockerkeithics/bitswarm-server) and [`dockerkeithics/bitswarm-web`](https://hub.docker.com/r/dockerkeithics/bitswarm-web) are published for `linux/amd64`, `linux/arm64`, `linux/arm/v7` and `linux/386`. Two Alpine-based containers run: `server` (the Go service with the BitTorrent engine) and `web` (nginx serving the dashboard and proxying `/api` to the server).

- **Sign-in is always required** in Docker, because the server sees nginx as the client and not localhost.
- **The web port is bound to `127.0.0.1` by default.** To reach it from other machines, set `SWARM_BIND=0.0.0.0` and put it behind HTTPS.
- **Data:** the database and engine state live in `./data` (change it with `SWARM_DATA_DIR`). Downloads go to `./downloads` (change it with `SWARM_DOWNLOADS_DIR`). Both folders must be writable by user ID 10001, which is what the `chmod` step does. Without it, Docker creates them owned by root and the server can't start on Linux.
- **Peers:** port 51413 (TCP and UDP) is published for peer connections. If that port is taken, set `SWARM_PEER_PORT` (for example `51414`), set the same listening port in Settings, and restart. The host and container ports must match so peers can reach you. UPnP can't open ports from inside a container, so forward the port on your router if you need incoming connections.
- **Versions:** the compose file pulls the latest stable release. To pin a version, set `SWARM_VERSION` (for example `SWARM_VERSION=0.9.1`) and run `docker compose pull && docker compose up -d` to update. Each release also has a `docker-compose.yml` already pinned to its version. Per-platform tags such as `0.9.1-amd64` exist too.
- **Other options** (set in your shell or a `.env` file): `SWARM_WEB_PORT` (default 8080), `SWARM_PEER_PORT` (default 51413), `SWARM_DATA_DIR`, `SWARM_DOWNLOADS_DIR`, `SWARM_DEMO`, `SWARM_ALLOW_COMMANDS`, `SWARM_UPDATE_FEED`, `LOG_LEVEL`. Commands run by hooks would run inside the server container.
- `docker compose down` keeps your data. Your `data` and `downloads` folders are not touched.
- **Build the images yourself** from this source instead of pulling them:

  ```sh
  docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build
  ```

### Release downloads

Each [GitHub release](https://github.com/bitswarm-dev/bitswarm/releases) has server binaries for Linux (amd64, arm64, armv7, 386), macOS (amd64, arm64) and Windows (amd64, arm64), the built dashboard, and `SHA256SUMS`. Check the download against `SHA256SUMS` before you run it.

The server binary serves the API on port 8080. It doesn't serve the dashboard. Put the dashboard files behind any web server that proxies `/api` to the server. The Docker setup does this for you with nginx.

### Run from source

You need [Go](https://go.dev/dl/) 1.26 or newer and [Node.js](https://nodejs.org/) with npm.

```sh
./run.sh
```

Then open <http://localhost:5173>. The script builds the server, starts it on port 8080, starts the dashboard (Vite proxies `/api` to the server) and stops both when you press Ctrl-C.

A fresh database is empty. Downloads go to `server/downloads`, or the folder in `SWARM_DOWNLOADS`. Sign-in is skipped on localhost by default (Settings → Web UI). When it's on, the default login is `admin` / `swarm`.

### Demo mode

```sh
./run.sh --demo
```

Demo mode uses a **simulated** engine with 10 sample torrents. Nothing is downloaded, and the interface says so.

## Configuration

The server reads these environment variables:

| Variable | Default | What it does |
|---|---|---|
| `PORT` | `8080` | Server port |
| `DB_PATH` | `data/swarm.db` | SQLite file. The engine keeps its state in an `engine/` folder next to it. |
| `SWARM_DOWNLOADS` | `./downloads` | Default save folder |
| `SWARM_DEMO` | off | `1` switches to the simulated engine with sample data |
| `SWARM_ALLOW_COMMANDS` | off | `1` lets command hooks run on this machine |
| `SWARM_UPDATE_FEED` | none | URL of a release feed for the update check |
| `LOG_LEVEL`, `LOG_FORMAT` | | Logging |

Everything else (save paths, limits, queueing, Web UI login) is in the Settings page.

## Not supported yet

The interface tells you where these apply:

- Per-torrent speed limits, sequential download, and first and last pieces first. The server refuses them.
- Skip hash check. Existing files are always verified.
- Tracker status. The Trackers tab lists URLs only.
- Applying port, UPnP, DHT and encryption changes without a restart. Connection limits, upload slots, proxy, PEX, LSD, uTP and the Web UI port, HTTPS and proxy settings are saved but not used yet.
- Automated tests for the dashboard. The Go side has tests, including a loopback peer-to-peer engine test.

## Contributing

Issues and pull requests are welcome. For anything bigger than a small fix, open an issue first so we can agree on the approach.

```
run.sh              starts the server and the dashboard
docker-compose.yml  runs the published images (docker-compose.build.yml builds them from source)
CLAUDE.md           rules for people and AI agents working on the code
server/             Go service: Fiber, SQLite, built-in BitTorrent engine
dashboard/          React 18, TypeScript, Vite, TanStack Query
```

Read [`CLAUDE.md`](./CLAUDE.md) first. It has the conventions, the event catalog and the design rules. Before you open a pull request, make sure these pass:

```sh
cd server && go build ./... && go vet ./... && go test ./... -race
cd dashboard && npx tsc -b --noEmit && npx vite build
```

UI changes also need a check in a real browser, in light and dark themes, at desktop and phone width. For end-to-end checks against real torrents and a real website (Debian's netinst images, with checksums), see "Testing the real engine" in [`CLAUDE.md`](./CLAUDE.md). Run test servers with their own `DB_PATH` and `SWARM_DOWNLOADS`, and delete your test data afterwards.

## Security

Bitswarm can run commands and call URLs through hooks, so treat access to it like shell access to the host. Change the default password, keep the port on localhost or behind HTTPS, and leave command hooks off unless you need them.

To report a vulnerability, open a private security advisory on the GitHub repository. Please don't post it in a public issue.

## License

Bitswarm is licensed under the [PolyForm Noncommercial License 1.0.0](./LICENSE). You can use it, change it and share it for free for any noncommercial purpose: personal use, hobby projects, research and study, and use by charities, schools and public institutions. You can't sell it or use it for commercial purposes, including running it inside a for-profit business.

This is a source-available license, not an OSI-approved open source license, because it limits commercial use. If you want to use Bitswarm commercially, open an issue to ask.

Third-party libraries keep their own licenses. They are listed in [THIRD_PARTY_NOTICES.md](./THIRD_PARTY_NOTICES.md), which also ships in the release downloads.
