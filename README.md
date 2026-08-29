# blocky-privdrop

A wrapper build of [blocky](https://github.com/0xERR0R/blocky) that drops
root privileges before the DNS server starts, keeping only the
capabilities it needs to run.

blocky needs to bind to port 53 (a privileged port), which normally
requires root. This wrapper starts as root, then re-executes blocky as
the unprivileged `nobody` user (`uid`/`gid` 65534) with a single ambient
capability: `CAP_NET_BIND_SERVICE`.

## Why this matters on gokrazy

[gokrazy](https://gokrazy.org/) starts and supervises every program as
root — there is no systemd with `User=`/`AmbientCapabilities=` units to
delegate privilege dropping to. Stock blocky doesn't drop privileges, so
on gokrazy it runs with full control of the appliance: one bug in blocky
means total compromise, including the persistent `/perm` partition.

This wrapper bounds a compromise to a single DNS process: blocky runs as
`nobody` with only `CAP_NET_BIND_SERVICE`, so it can bind port 53 but
cannot write the filesystem or touch other services. It follows the
pattern recommended in gokrazy's
[privilege dropping docs](https://gokrazy.org/development/process-interface/index.html#privilege-dropping--security).

## How it works

On startup (`main.go`):

1. If `BLOCKY_PRIVILEGES_DROPPED` is not set, the process assumes it is
   running as root:
   - Reads the current capabilities (`SYS_CAPGET`)
   - Adds `CAP_NET_BIND_SERVICE` to the permitted and inheritable sets
     (`SYS_CAPSET`) so it can be raised into the ambient set
   - Re-executes itself as `nobody:nogroup` (65534:65534) with
     `CAP_NET_BIND_SERVICE` as an ambient capability and
     `BLOCKY_PRIVILEGES_DROPPED=1` in the environment
2. On the re-executed run, it verifies:
   - The UID and GID are 65534
   - `CAP_NET_BIND_SERVICE` is effective and permitted
3. Finally, it hands control to blocky's CLI (`cmd.Execute()`)

If any verification step fails, the process exits instead of running
blocky with unexpected privileges.

## Usage

Build and run as root (a supervisor such as gokrazy, systemd, or a
container entrypoint works fine):

```sh
go build -o blocky-privdrop .
./blocky-privdrop serve --config /path/to/config.yml
```

All arguments are passed through to blocky unchanged. The binary must
start as root (or with `CAP_NET_SETGID`, `CAP_NET_SETUID`, and
`CAP_SETCAP` capabilities) so it can drop privileges.

## Requirements

- Linux (uses Linux capability syscalls)
- Go (see `go.mod` for the exact version)

## Development

A Nix flake is provided with a dev shell:

```sh
nix develop
```
