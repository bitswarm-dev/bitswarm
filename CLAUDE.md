# CLAUDE.md — Bitswarm

Instructions for AI coding agents (and humans) working on **Bitswarm**, a web UI for a BitTorrent client. Read this file before changing anything. For what Bitswarm does, see the feature list in [`README.md`](./README.md). Rules for the Go service live in [`server/CLAUDE.md`](./server/CLAUDE.md).

---

## 1. What this project is

Bitswarm is a browser-based interface for managing torrents, in the style of qBittorrent, Transmission and Deluge web UIs, with extra automation: scheduled site searches, regex rename presets, maintenance tasks, developer hooks, an execution log, API tokens and settings backup.

**Current state: a React dashboard on a standalone Go server with a built-in BitTorrent engine.**

The engine is **real and built in**: `server/internal/engine` wraps `github.com/anacrolix/torrent`, so Bitswarm downloads and seeds by itself with no qBittorrent, Transmission or other client. The old simulation only runs in demo mode (`SWARM_DEMO=1`). Every place where something is faked or unsupported must say so in the UI.

| | |
|---|---|
| Dashboard | `dashboard/`: Vite, React 18, TypeScript, React Router, TanStack Query v5 |
| Server | `server/`: Go, Fiber, SQLite (`modernc.org/sqlite`), module `bitswarm/server` |
| Run everything | `./run.sh`, then open http://localhost:5173 (server on :8080, Vite proxies `/api`). A fresh database is empty and downloads for real into `server/downloads` (or `SWARM_DOWNLOADS`). `./run.sh --demo` (or `SWARM_DEMO=1`) switches to the **simulated** engine and adds 10 sample torrents. |
| Docker (preferred for users) | `docker compose up -d` pulls `dockerkeithics/bitswarm-server` and `bitswarm-web` (`SWARM_VERSION` pins a version, default `latest`). `docker-compose.build.yml` builds them from source. |
| Demo sign-in | `admin` / `swarm` (skipped on localhost while "Skip sign-in on localhost" is on, which is the default) |

---

## 2. Port status

Every feature has a server API and a dashboard screen. Keep this table honest.

| Area | Status |
|---|---|
| Sign-in, lockout, sessions, localhost bypass, legal notice | done |
| Layout, sidebar, status bar, theme | done |
| Torrent list, toolbar, context menu, add dialog, inspector, tags | done. The right-click menu is rendered in a portal on `document.body`, because the blurred page panel would otherwise offset `position: fixed`. |
| Folder checks (add, move, settings, schedules) | done: `pathcheck.Ensure` creates the folder and writes a probe file |
| Rename presets | done (Go regex syntax, not JavaScript: no lookahead or backreferences). File targets may put `/` in the replacement to move files into a folder (created if missing; `..` dropped); torrent names still turn `/` into `-`. A name clash on disk is numbered like Finder ("(2)"), never overwritten (`Engine.SetNames` returns the final names). |
| Scheduled downloads | done: timed runs fetch live URLs and were run end to end on the real Debian page; previews fetch the live search URL. A rule has a search URL with an optional `{query}` and search terms (one search each). Fetches are spaced by Settings → "Wait between searches" (default 5 s, `searchDelay`); the name is optional and defaults to the first term. There is no pasted sample: the editor preview fetches live. An empty "Save to" means the default folder from Settings. Rules apply tags and can pick or create a rename preset inline. |
| Maintenance | done |
| Hooks | done: API hooks are really sent; command hooks run only with `SWARM_ALLOW_COMMANDS=1` |
| API tokens | done: Bearer tokens with read, control and admin scopes, also under `/api/v1` |
| Backup export and import | done |
| Execution log, settings, about | done |
| Real BitTorrent engine | done: magnet and `.torrent` adds, real file lists, per-file priorities, pause/resume/recheck, move and rename on disk, queueing, global limits, seeding limits, restart recovery |
| Update check | done, but needs a feed: `SWARM_UPDATE_FEED`, or `SWARM_DEMO=1` for a sample feed |

Scoping and defaults (all done):
- **Tags are the only grouping.** Labels were removed. Rename presets, maintenance tasks and hooks are scoped by one tag ("Only for tag"); schedules apply tags to what they add. Old saved `label` fields are ignored.
- **System defaults:** schedules and the Add dialog take the save folder, start behavior and content layout (subfolder or not) from Settings unless the rule sets its own. The save folder follows later changes to the default. "Start right away" is copied into a new rule and then fixed.

Known gaps (all disclosed in the UI):
- The dashboard has no automated tests (the Go side does, including a loopback peer-to-peer engine test).
- **Per-torrent speed limits**, **sequential download** and **first and last pieces first** aren't supported by the engine library. The server refuses them with a clear message. Global limits and the alternative schedule work.
- **Skip hash check** is ignored: files already on disk are always verified.
- Tracker status (working, next announce) isn't reported, so the Trackers tab lists URLs only. Web-seed transfers don't appear in the Peers tab.
- Port, UPnP, DHT and encryption are read at startup (restart after changing). Connection limits, upload slots, proxy, PEX, LSD, uTP and the Web UI port/HTTPS/proxy settings are saved but unused.
- The simulated engine (demo mode) still fakes peers, trackers and availability.

---

## 3. Golden rules

1. **Never pretend.** If something is simulated (downloads, update feed, hook delivery, command execution), the UI must say so in plain words. Hooks log as **"Not sent" / "Not run"** until delivery truly works.
2. **Every user-visible lifecycle step goes through `events.Emitter.Emit()`** (§6). That one call feeds the execution log and the hooks.
3. **Safety first for anything that runs or leaves the machine.** Shell placeholders are always single-quoted, secrets are hashed, masked or kept in the environment, hooks show an advanced-user warning, imported hooks start disabled, and command hooks stay off unless the server is started with them enabled.
4. **Both themes, both widths.** Every UI change must work in light and dark (system, light, dark) and at ~390 px width with no horizontal page scroll.
5. **Plain, specific copy.** Buttons say exactly what happens ("Revoke", "Move", "Save changes"). Errors say what went wrong and how to fix it. No apologies, no vague "Something went wrong".
6. **Reuse the existing look.** `dashboard/src/styles/global.css` holds the shared CSS. Use its class names (`.row`, `.fbtn`, `.pill`, `.sched`, `.rule`, `.modal`, `.menu`, …) before inventing new ones, and keep colors as tokens.
7. **Test in a real browser before you say it works** (§9).
8. **Comments in code only when asked.** No inline comments by default.

---

## 4. Layout

```
run.sh                  starts server + dashboard, stops both on exit
server/                 Go service (see server/CLAUDE.md)
  cmd/server/main.go    composition root
  internal/
    auth/               password (PBKDF2), sessions, lockout, localhost bypass
    backup/             settings export and import (parts: settings, theme, tags, schedules, presets, maintenance, hooks)
    catalog/            the list of tag names, plus add, rename, merge and delete across torrents, schedules, presets, maintenance and hooks (each implements `catalog.Owner`)
    config/             env config: PORT, LOG_LEVEL, LOG_FORMAT, DB_PATH, SWARM_DEMO, SWARM_DOWNLOADS
    db/                 SQLite open + embedded migrations (migrations/*.sql, applied in order)
    docstore/           generic JSON document collections and a key-value table
    eventlog/           execution log; Describe() turns events into sentences
    events/             Emitter interface and fan-out Bus
    health/             worked example from the Go skill
    hooks/              API and command hooks: placeholders, signing, retries, delivery log
    http/               Fiber app, middleware (request id, recover, logging)
    maintenance/        scheduled housekeeping tasks (scoped by tag) and log pruning
    pathcheck/          Ensure(): creates a folder and writes a probe file
    renames/            regex rename presets (scoped by tag), apply, undo, preview
    schedules/          timed site searches, link extraction, seen list, preset hookup
    settings/           Settings struct, validation, save
    tokens/             API tokens (read, control, admin)
    updates/            update check against a feed
    torrents/           model, Engine interface, simulation (demo), real-engine sync (real.go), add flow, batch actions, API
    engine/             the real BitTorrent engine: anacrolix/torrent client, storage layout, rename/move on disk
dashboard/
  src/app/              main.tsx, App.tsx (routes), Layout.tsx, theme.tsx, ui.tsx (UI-only context), storage.ts
  src/components/       Sidebar, StatusBar, Toasts, Modal, Menu, Charts, Icons, Theme, PageQuery, PageSkeleton
  src/features/<name>/  types.ts, api.ts, queries.ts, mutations.ts (+ filters.ts, sections.ts where needed)
  src/pages/<name>/     route pages and their dialogs
  src/lib/http.ts       fetch wrapper, ApiError
  src/utils/            format.ts, safe.ts, tags.ts
```

### Dashboard conventions

- **Server state is TanStack Query only.** Queries use `useSuspenseQuery`. Mutations invalidate the keys they affect.
- **Suspense at the route.** Every route page is wrapped in `<Suspense>` in `app/App.tsx` with a skeleton that matches the page. Pages never check `isLoading`.
- **`PageQuery`** renders query results and empty states. Pages pass the query and render only the success UI.
- **React Context is UI state only**: theme, toasts, sidebar collapse, list filter/sort/selection. No server data in Context.
- **Check `src/utils/` before writing an inline `typeof` or `Array.isArray` guard.** Add a helper there if a pattern repeats.
- **Focus-sensitive controls** (selects, inputs) are fine in React lists because rows keep their keys. Keep keys stable.
- Polling: torrents and stats refetch every second, the catalog every five, the log every two.

### Server conventions

See `server/CLAUDE.md`. In short: handler, then service, then repository. Services depend on a `Repository` interface. One test file per source file, table-driven, stdlib only. Config-like data goes through `docstore`; add an `NNNN_name.sql` migration only for new tables.

---

## 5. State and persistence

| Data | Where |
|---|---|
| Torrents (with files, peers, history) | SQLite `docs` table, collection `torrents` |
| Engine state | `<dir of DB_PATH>/engine/`: `bolt` piece-completion database and `torrents/<hash>.torrent` copies, so a restart resumes without re-checking |
| Settings | SQLite `kv`, key `settings`. `webPass` is never stored in it. |
| Password | SQLite `kv`, key `auth`: PBKDF2-SHA256 hash and salt. Absent means the demo password. |
| Tag names | SQLite `kv`, key `catalog` |
| Execution log | SQLite `docs`, collection `log`, trimmed to about 1,000 entries |
| Sessions | Server memory (a restart signs everyone out) |
| Theme | Browser `localStorage` `swarm-theme` (photo is a downscaled JPEG data URL; on quota error it is saved without the photo) |
| Sidebar collapsed groups | Browser `localStorage` `swarm-sidebar` |
| Legal notice / hooks warning dismissed | Browser `localStorage` `swarm-legal-ack`, `swarm-hooks-ack` |

Browser storage can throw or be empty (private mode). Wrap every read and write in `try/catch` (see `app/storage.ts`) and make the page work without it. About → **Reset saved data** clears only browser keys. It does not touch the server.

Downloaded data lives in each torrent's own save folder (default `server/downloads`). Removing a torrent with "delete data" deletes its files and the empty folders inside its save folder, never the save folder itself.

---

## 6. Events: the log and hooks

Every lifecycle step calls `Emit(event, data)` on the shared `events.Bus`. Two subscribers listen on it: the execution log (`eventlog.Service`) and the hooks (`hooks.Service`).

**Event catalog.** Keep `eventlog.Describe` and this table in sync when adding one.

| Group | Events |
|---|---|
| Torrents | `torrent.added`, `torrent.metadata`, `torrent.resumed`, `torrent.paused`, `torrent.completed`, `torrent.error`, `torrent.removed`, `torrent.renamed`, `torrent.tagged`, `torrent.moved` |
| Schedules | `schedule.search`, `schedule.links`, `schedule.finished` |
| App | `maintenance.run`, `settings.saved`, `update.available` |

Every event in the table is emitted today. Use `Logger.Add(level, source, message)` directly for things that aren't hook events: sign-in, queue moves, rechecks, limits, boot info. Levels: `error`, `warn`, `info`, `debug` (debug is hidden by default; hook deliveries log at debug).

**Torrent payloads** come from `torrents.Info(t)`: `{id, hash, name, tags, size, size_text, progress, ratio, state, save_path}`. Add fields there, never ad hoc.

### Hooks

- Placeholders: `{{payload}}`, `{{event}}`, `{{time}}`, `{{torrent.name}}`, `{{torrent.hash}}`, `{{torrent.tags}}`, `{{torrent.size}}`, `{{torrent.bytes}}`, `{{torrent.path}}`, `{{torrent.ratio}}`, `{{torrent.state}}`, `{{schedule.name}}`, `{{task.name}}`, `{{count}}`.
- Escape by context: URL gets `encodeURIComponent`; JSON gets JSON-string escaping (except `payload`, inserted raw); shell gets single quotes; text is raw.
- **Secrets:** `{{secret.NAME}}` is resolved only at send time, from `SWARM_SECRET_NAME` or `SWARM_SECRET_NAME_FILE`. `Fill` swaps the hook author's reference for a private marker that includes a random per-process nonce, and only that marker is resolved, so event data such as a torrent name can never pull a secret in (`{{secret.X}}` in a torrent name stays plain text). `maskDelivery` turns the marker back into `{{secret.NAME}}`, so previews, stored deliveries, logs and backups only ever hold the reference. Never resolve a secret by scanning text that event data has been filled into. Names are `[A-Z][A-Z0-9_]*` everywhere: `validate` rejects anything else in a `{{secret.…}}` reference, `Fill` emits no marker for it, and the sender refuses any request that still holds a marker after resolving. Build error text from the unresolved URL (`safeError`), never from the URL with secrets filled in. API hooks only: `validate` rejects it in commands. Header values for `Authorization`, `X-Api-Key` and similar (`IsSecretHeader`) are masked as `********` in the view, previews and stored deliveries, and `Update` restores the saved value when the mask comes back unchanged. Backups blank real values in those headers and keep `{{secret.NAME}}` references. Never put a resolved secret in a log line, an error or a `Delivery`.
- **curl import:** `POST /api/hooks/parse-curl` (`ParseCurl` in `internal/hooks/curl.go`) turns a pasted curl command into method, URL, headers and body for the hook form. It is pure text parsing with its own small shell tokenizer: it never runs a shell, expands `$(…)` or `$VAR`, reads a file (`@file` is skipped with a warning) or makes a request, and its warnings and errors never repeat a header value. It accepts only POST, PUT, PATCH and GET and drops headers Bitswarm sets itself. Keep it that way when adding options.
- **URLs in errors:** run client errors through `redact.Err` so query strings (`?apikey=…`) never reach the log or a stored delivery.
- API requests add `User-Agent: Bitswarm/<version>` and `X-Swarm-Event`. With a signing secret they add `X-Swarm-Signature: sha256=<hex HMAC-SHA256 of the body>`.
- Commands get env vars `SWARM_EVENT`, `SWARM_TIME`, `SWARM_TORRENT_NAME`, `SWARM_TORRENT_HASH`, `SWARM_TORRENT_PATH`, `SWARM_TORRENT_SIZE`, `SWARM_SCHEDULE`, `SWARM_TASK`, optional JSON on stdin, and a timeout.
- **Never** interpolate a placeholder into a shell command without single-quoting it. A torrent name like `x'; rm -rf ~ #` must arrive as one literal argument.

---

## 7. Design system

- **Look**: calm, utilitarian client UI. Mint accent by default; neutrals with a cool blue bias; semantic colors for download (blue), upload/OK (green), warning (amber), error (red), idle (grey). A background image layer sits behind translucent panels.
- **Tokens only.** Colors are CSS custom properties on `:root`, redefined under `@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) {…} }` and `:root[data-theme="dark"] {…}`. Never hard-code a color in a component. Accent comes from `ACCENTS` in `app/theme.tsx`.
- **Type**: Bricolage Grotesque (headings), Figtree (text), JetBrains Mono (numbers, paths, hashes, regex). Use `.num` for aligned digits.
- **Reusable patterns**: `.sched` page container with `.sched-head`, `.note` banner, `.rule` card, `.switch`, `.pill` + `.st-*`, `.chip`, `.tag`, `.flag`, `.preview`, `.log`, `.confirm` inline confirmation, `.modal` (`.wide`) with `.step` fieldsets, `.field`, `.row2`, `.row3`, `.check`, `.sfield`, `.menu` with `.lopt`, `.seg`, `pre.req`.
- **Sidebar order**: Status, Automation (Scheduled, Auto rename, Maintenance), Tags, Developer (Execution log, API tokens, Hooks), speed chart, then Settings and About pinned at the bottom. Groups collapse and remember state.
- **Toolbar order**: Add torrent, Search, Resume, Pause, Remove, Tags, Rename, More, selection count, (spacer) Speed limit.
- **One scrollbar**: `body` has `margin:0` (the default 8px margin made the page overflow and added a second scrollbar). On windows shorter than 760 px the page flows and scrolls as a whole, with no inner panel scrolling; on taller windows only the panels scroll.
- **Responsive**: at ≤820 px the list becomes cards, sidebar groups become chip rows, toolbar buttons become icons except Add torrent.
- **Accessibility**: real buttons, `aria-pressed`/`aria-expanded`/`aria-checked`, roles on menus, labels on every input, visible focus, Escape closes the topmost dialog or menu, `prefers-reduced-motion` honored.

---

## 8. Copy and legal text

- Disclaimer language exists in the legal notice after sign-in, the hooks warning, About → Disclaimer, and the Hooks page note. Keep them consistent: Bitswarm isn't built for piracy, doesn't provide or index content, users are responsible for following their local laws, and the author and developers aren't liable for use, content, or damage from hooks and automation.
- These texts are product copy, **not legal advice**. If they change materially, flag it for a human or legal review.
- Writing style: active voice, sentence case, name things the way users recognize them ("Download finished", not `torrent.completed`, except where the technical name sits next to it).

---

## 9. How to test

1. **Server**: `cd server && go build ./... && go vet ./... && go test ./... -race`.
2. **Dashboard**: `cd dashboard && npx tsc -b --noEmit && npx vite build`.
3. **Drive it in a real browser.** Start `./run.sh`, then use Playwright or Chrome automation against http://127.0.0.1:5173. Capture `pageerror` and console errors. Exercise the feature you touched end to end, including error paths. To test the sign-in page, turn off "Skip sign-in on localhost" in Settings → Web UI first, and turn it back on afterwards.
4. **Screenshot** at 1360×900 and 390×844, light and dark, and look at them: clipped text, overlap, horizontal scroll, unreadable contrast.
5. Check that the new step appears in the **execution log** (and, for hook events, in Recent deliveries).

Useful selectors: rows `.row[data-id="N"]`, checkboxes `#chk-N`, settings fields `#set-<key>`, toolbar `.tools button`. Scope button selectors (`.tools button:has-text("Pause")`) because the sidebar also has "Paused".

---

### Testing the real engine

- `go test ./internal/engine -race` runs two in-process clients on loopback (a seeder and the engine), so it needs no internet.
- **Call `Engine.Live` once per torrent per tick.** Each call consumes the byte counters, so extra calls zero the speed chart and session totals. Use `Engine.Loaded` for "is it in the engine?" checks. `TestService_TickReal_ReadsLiveOncePerTorrent` guards this.
- For an end-to-end run use public, legal torrents and check the result against the publisher's checksum, for example the Debian netinst `.torrent` files under `https://cdimage.debian.org/debian-cd/current/amd64/bt-cd/` (compare with `SHA256SUMS` in `https://cdimage.debian.org/debian-cd/current/amd64/iso-cd/`).
- **Schedule end to end:** create a rename preset and a schedule through the API (search URL set to the Debian page, selector `a[href$='.torrent']`, link type torrent, include `netinst`, exclude `mac,edu,firmware`, empty path). Call `POST /api/schedules/1/run`, wait for seeding, then compare the SHA-256 of the renamed ISO with `SHA256SUMS`. A second run must log "nothing new".
- Test a multi-file torrent too (for example `https://archive.org/download/BigBuckBunny_328/BigBuckBunny_328_archive.torrent`) for file selection and priorities.
- **Never delete `server/data` or `server/downloads`: they hold the user's real schedules, presets and downloads.** Run test servers with their own `DB_PATH` and `SWARM_DOWNLOADS` in a temp or scratchpad folder, on other ports, and delete only those afterwards. Remove test torrents with "delete data" only from that test instance.

---

## 10. Roadmap

### Next
Close the known gaps in §2: per-torrent speed limits, sequential download, tracker status, applying connection settings without a restart, and dashboard tests.

### Discussed, not built
RSS feeds with auto-download rules · create torrent · built-in multi-site search · IP blocklist, ban/add peers · statistics page · bulk add · column chooser and compact density · tracker editing per torrent · two-factor sign-in and multiple user accounts.

### Explicitly dropped
**Labels.** Tags cover the same need, so labels were removed. Presets, maintenance tasks and hooks are scoped by a tag, and schedules apply tags to what they add. Don't bring labels back unless asked.

**AI auto-labeling** (considered with a small embedding model; the user decided against it). Don't add it unless asked.

---

## 11. Releasing

Releases are manual: `main` is always releasable and a version tag on it publishes the release. The full steps are in [`docs/releasing.md`](./docs/releasing.md). Branches are where you work, tags are what ships. Never move or reuse a tag.

**The tag is the only source of the version.** There is nothing to bump. The pipeline passes the tag's version into the server (`-ldflags` setting `internal/buildinfo.Version`), the dashboard (`VITE_APP_VERSION`) and the Docker images, and pins the `docker-compose.yml` attached to the release. A local build reports `dev`. Don't hard-code a version number in code.

Pushing `vX.Y.Z` or `vX.Y.Z-rc.N` runs `.github/workflows/docker-release.yml`. It refuses a tag that isn't in that form, or whose commit isn't on `main` or on `release/X.Y`. Create a `release/X.Y` branch only to patch an older version.
