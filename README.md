# Bitswarm

A standalone BitTorrent client with a web interface. Bitswarm downloads and seeds by itself using a built-in engine, so you don't need qBittorrent, Transmission or any other client.

On top of the usual torrent list it adds automation: scheduled searches of websites, regex rename presets, maintenance tasks, hooks that call an API or run a command, an execution log, API tokens and settings backup.

> Bitswarm is a general-purpose BitTorrent client. It isn't built for piracy, and it doesn't provide, host or index content. Only download and share what you have the right to. You are responsible for following your local laws. This is not legal advice.

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

## Quick start

**Docker is the preferred way to run Bitswarm; see [Docker](#docker-recommended) below.** To run it from source for development, you need [Go](https://go.dev/dl/) 1.26 or newer and [Node.js](https://nodejs.org/) with npm.

```sh
./run.sh
```

Then open <http://localhost:5173>. The script builds the server, starts it on port 8080, starts the dashboard (Vite proxies `/api` to the server) and stops both when you press Ctrl-C.

A fresh database is empty. Downloads go to `server/downloads`, or the folder in `SWARM_DOWNLOADS`.

Sign-in is skipped on localhost by default (Settings → Web UI). When it's on, the default login is `admin` / `swarm`. Change the password in Settings → Web UI.

### Docker (recommended)

Docker is the easiest way to run Bitswarm: no Go or Node.js, and one command. You only need [Docker](https://docs.docker.com/get-docker/) with Compose.

```sh
mkdir bitswarm && cd bitswarm
curl -fsSLO https://raw.githubusercontent.com/keithics/bitswarm/main/docker-compose.yml
mkdir data downloads && chmod 777 data downloads
docker compose up -d
```

This pulls the ready-made images [`dockerkeithics/bitswarm-server`](https://hub.docker.com/r/dockerkeithics/bitswarm-server) and [`dockerkeithics/bitswarm-web`](https://hub.docker.com/r/dockerkeithics/bitswarm-web) and starts them. You only need `docker-compose.yml` from this repository, which the commands above download into an empty folder. The `chmod` step lets the container (user ID 10001) write to both folders; without it, Docker creates them owned by root and the server can't start on Linux. Then open <http://localhost:8080>. Two Alpine-based containers run: `server` (the Go service with the BitTorrent engine) and `web` (nginx serving the dashboard and proxying `/api` to the server).

- **Sign-in is always required** in Docker, because the server sees nginx as the client and not localhost. The default login is `admin` / `swarm`. **Change it right away** in Settings → Web UI before exposing the port beyond your machine.
- **The web port is bound to `127.0.0.1` by default.** To reach it from other machines, set `SWARM_BIND=0.0.0.0` and put it behind HTTPS.
- **Data:** the database and engine state live in the `data` folder next to `docker-compose.yml` (change it with `SWARM_DATA_DIR`). Downloads go to the `downloads` folder next to `docker-compose.yml`. To use another folder, set `SWARM_DOWNLOADS_DIR` (both folders must be writable by user ID 10001).
- **Peers:** port 51413 (TCP and UDP) is published for peer connections. If that port is taken, set `SWARM_PEER_PORT` (for example `SWARM_PEER_PORT=51414`), set the same listening port in Settings, and restart. The host and container ports must match so peers can reach you. UPnP can't open ports from inside a container, so forward the port on your router if you need incoming connections.
- **Versions:** the compose file uses the pinned `0.9.0` images. Set `SWARM_VERSION=latest` (or another version) to change that, then run `docker compose pull && docker compose up -d` to update. Your data stays in the `data` and `downloads` folders.
- **Other options** (set in your shell or a `.env` file): `SWARM_WEB_PORT` (default 8080), `SWARM_PEER_PORT` (default 51413), `SWARM_DATA_DIR`, `SWARM_DOWNLOADS_DIR`, `SWARM_DEMO`, `SWARM_ALLOW_COMMANDS`, `SWARM_UPDATE_FEED`, `LOG_LEVEL`. Commands run by hooks would run inside the server container.
- `docker compose down` keeps your data. Your `data` and `downloads` folders are not touched.
- **Build the images yourself** from this source instead of pulling them:

  ```sh
  docker compose -f docker-compose.yml -f docker-compose.build.yml up -d --build
  ```

### Run from source

You need [Go](https://go.dev/dl/) 1.26 or newer and [Node.js](https://nodejs.org/) with npm. This is meant for development; see Quick start above.

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

## Project layout

```
run.sh          starts the server and the dashboard
docker-compose.yml  runs the published images (docker-compose.build.yml builds them from source)
draft.html      the original single-file prototype (visual reference)
CLAUDE.md       rules for people and AI agents working on the code
server/         Go service: Fiber, SQLite, built-in BitTorrent engine
dashboard/      React 18, TypeScript, Vite, TanStack Query
```

## Development

```sh
cd server && go build ./... && go vet ./... && go test ./... -race
cd dashboard && npx tsc -b --noEmit && npx vite build
```

For the end-to-end checks against real torrents and a real website (Debian's netinst images, with checksums), see "Testing the real engine" in [`CLAUDE.md`](./CLAUDE.md). Delete your test data afterwards.

## License

No license has been chosen yet, so all rights are reserved until one is added.
