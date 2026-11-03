# Contributing to Bitswarm

Thanks for helping. Bug reports, ideas, docs fixes and code are all welcome.

## Before you start

- **Small fixes** (typos, copy, a clear bug): open a pull request.
- **Anything bigger:** open an issue first so we can agree on the approach before you spend time on it.
- **Security problems:** don't use a public issue. See [SECURITY.md](./SECURITY.md).
- **Not supported yet:** the README lists known gaps. A request for one of those is a feature request, not a bug.

## Set up

You need [Go](https://go.dev/dl/) 1.26 or newer and [Node.js](https://nodejs.org/) with npm.

```sh
./run.sh            # real engine, downloads into server/downloads
./run.sh --demo     # simulated engine with sample torrents
```

Then open <http://localhost:5173>. Use your own `DB_PATH` and `SWARM_DOWNLOADS` for test servers, so your real data stays untouched.

## Conventions

[`CLAUDE.md`](./CLAUDE.md) is the rulebook for people and AI agents. The short version:

- The server is layered: handler, then service, then repository. See [`server/CLAUDE.md`](./server/CLAUDE.md).
- The dashboard uses TanStack Query for server state, Suspense at the route, and the existing CSS classes and color tokens.
- Every user-visible step goes through the event emitter, so it shows in the execution log and the hooks.
- Never pretend: if something is simulated or unsupported, the interface says so in plain words.
- Comments in code only when they're needed. No inline comments by default.
- Copy is plain and specific: buttons say what they do, errors say what went wrong and how to fix it.

## Checks before you open a pull request

```sh
cd server && go build ./... && go vet ./... && go test ./... -race
cd dashboard && npx tsc -b --noEmit && npx vite build
```

For interface changes, also check in a real browser in the light and dark themes, at desktop width and at about 390 px, with no horizontal page scroll. Add a screenshot to the pull request.

## Pull requests

- Target `main`. Only a patch for an older version goes to a `release/X.Y` branch. See [docs/releasing.md](./docs/releasing.md).
- One topic per pull request, with a short description of what changed and why.
- Add or update tests for server changes (one test file per source file, table-driven, standard library only).
- Don't add a dependency without asking first.
- Keep commits readable. Squash noise before you ask for review.

## Contribution terms

By sending a contribution you confirm that you wrote it or have the right to send it. You keep your copyright. You also agree that:

1. your contribution is licensed under the project license, the [PolyForm Noncommercial License 1.0.0](./LICENSE), and
2. the maintainer may also use, change and license your contribution under other terms, including commercial licenses, with a perpetual, worldwide, royalty-free, irrevocable license from you.

If you can't agree to that, please don't send a contribution.

## Code of conduct

Everyone taking part follows the [Code of Conduct](./CODE_OF_CONDUCT.md).
