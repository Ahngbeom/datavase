# Security

## Reporting a vulnerability

Use GitHub's private reporting:
[**Report a vulnerability**](https://github.com/Ahngbeom/datavase/security/advisories/new).
It reaches the maintainer without the report being public first.

Please do not open a public issue for anything that lets someone reach data
or credentials they should not have.

**One maintainer, no rota.** Expect an acknowledgement in days rather than
hours, and say in the report if that is too slow for what you found.

## What is supported

The latest release. Fixes go into the next one; nothing is backported.
`dv version` prints what you are running, and
[CHANGELOG.md](CHANGELOG.md) says what changed between releases.

## What this program is, and is not

`dv` is a database client. It holds a connection and a credential on behalf
of the person running it, and it has exactly the privileges their database
account already has. It is not an access proxy, a bastion, a secrets manager
or an audit system, and it cannot be made into one from the client side.

Three claims in particular are narrower than they may read:

- **`read_only: true` is a guard against a slip, not a boundary.** The server
  is asked to make the session read-only and refuses to open the datasource
  if it will not confirm, so nothing the client failed to recognise as a
  write can get through. `SET SESSION TRANSACTION READ WRITE` lifts it for
  the session, and whoever types that has decided to. Database privileges are
  the boundary.
- **`auto_limit` bounds what is fetched, not what the server does.** A
  statement that scans the whole table still scans it.
- **Cancelling sends `KILL QUERY` on a second connection.** It is a request
  to the server, and the server decides.

## What reaches the network

The database connection, and the SSH connection to a bastion when one is
configured. Nothing else: no telemetry, no update check, no crash reporting.
A release is downloaded only when you run the install script or Homebrew.

TLS is set per datasource. An absent `tls:` means `preferred`, which encrypts
when the server offers it and silently does not when it does not — so it
proves nothing about who answered. `required` encrypts or fails; `verify-ca`
and `verify-identity` also check the certificate. Only the last two make the
connection worth trusting on a network you do not.

## What reaches the disk

| What | Where | Mode |
|---|---|---|
| Datasources | `$XDG_CONFIG_HOME/datavase/config.yaml`, or `~/.config/datavase/config.yaml` | file `0600` |
| Statement history | `$XDG_STATE_HOME/datavase/history.db`, or `~/.local/state/datavase/history.db` | directory `0700`, file `0644` |
| Schema cache | the same directory, `datavase.db` | directory `0700`, file `0644` |
| Exported results | wherever you name; an existing path is refused rather than replaced | file `0600` |

**The directory is what protects the two databases.** They are created
`0644` inside a `0700` directory, so another account on the machine cannot
reach them and another process running as you can.

**Passwords are never written to the configuration file.** They go to the OS
keychain through `dv auth`, or come from `DATAVASE_PASSWORD_<NAME>` in the
environment, which takes precedence when it is set. The environment is the
route for a machine with no keychain, and an environment variable is
readable by anything else running as you.

**History can be turned off.** `history: false` under `defaults` stops
statements being written at all; the file is not created. This is the setting
to reach for where the statements themselves are sensitive. Deleting the two
databases is safe at any time: the cache is rebuilt on the next reload and
the history is gone.

## Releases

Every archive and package is built by
[the release workflow](.github/workflows/release.yml) on a `v*` tag and
signed by it. To check one against the workflow that produced it:

```sh
gh attestation verify <the archive you downloaded> --repo Ahngbeom/datavase
```

That answers a different question from the checksum the install script
verifies: a checksum says the file arrived intact, this says who built it.

To pin or to go back to a version:

```sh
DV_VERSION=v0.10.0 curl -fsSL https://raw.githubusercontent.com/Ahngbeom/datavase/main/install.sh | sh
```

Homebrew always installs the newest release; the install script is the route
that takes a version. Every release stays on the
[releases page](https://github.com/Ahngbeom/datavase/releases), so going back
is downloading an older one — there is no state in a newer release that an
older one cannot read.

## macOS

The binaries are signed ad-hoc by the Go linker and notarized by nobody. The
install script and Homebrew both avoid the quarantine that would otherwise
stop them running; an archive taken with a browser needs
`xattr -dr com.apple.quarantine ./dv`, which is a step you should be
suspicious of and is documented rather than hidden.
