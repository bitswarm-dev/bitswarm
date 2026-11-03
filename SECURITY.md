# Security policy

## Supported versions

Only the latest release gets security fixes. Right now that is the 0.9 series.

## Report a vulnerability

Please don't open a public issue for a security problem.

Report it privately: open the **Security** tab of this repository and choose **Report a vulnerability**, or go to <https://github.com/bitswarm-dev/bitswarm/security/advisories/new>.

Include:

- what you found and where (page, API route or file)
- the Bitswarm version and how you run it (Docker, binary or source)
- steps to reproduce, or a proof of concept
- what an attacker could do with it

You'll get a first reply within 7 days. We'll agree on a fix and a disclosure date with you, and credit you in the release notes if you want that.

## What counts

Examples we care about:

- bypassing sign-in, API token scopes or the localhost rule
- running commands without `SWARM_ALLOW_COMMANDS=1`
- shell or template injection through hook placeholders
- reading or writing files outside a torrent's save folder
- leaking secrets such as password hashes, API tokens or hook signing secrets

## Running Bitswarm safely

- Change the default `admin` / `swarm` login right away in Settings → Web UI.
- Keep the web port on `127.0.0.1`, or put it behind HTTPS if you expose it.
- Leave command hooks off unless you need them. A command hook can run anything the server user can run.
- Treat API tokens like passwords. Prefer the read scope when control isn't needed.
