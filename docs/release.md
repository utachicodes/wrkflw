# Releases

Two release lines share the `v*` tag namespace. A tag matching
`frwrd/Cargo.toml` ships the gateway; any other `v*` tag ships the app.

## App: `vX.Y.Z` (CLI + server, same version)

```sh
git tag v2.1.0 && git push origin v2.1.0
```

`.github/workflows/release.yml` runs CLI tests, builds the web UI, then
publishes eight archives plus checksums:

- `wrkflw_<os>_<arch>.tar.gz` — the CLI
- `wrkflw-server_<os>_<arch>.tar.gz` — the control plane (UI embedded)

for `darwin`/`linux` × `amd64`/`arm64`. The `create-wrkflw`
downloader reads these exact names; do not rename them. Verify locally
anytime with `sh scripts/test-cli-release.sh` and
`sh scripts/test-server-release.sh` (both also run in CI).

## Gateway: `v<frwrd/Cargo.toml version>`

```sh
git tag v0.10.0 && git push origin v0.10.0
```

The same workflow validates the tag against `frwrd/Cargo.toml`
(`frwrd/scripts/check-release-version.sh`), builds natively on Ubuntu
and macOS runners, and publishes `frwrd-<tag>-<target>.tar.gz` with
`.sha256` sidecars plus `frwrd/RELEASE_NOTES.md`. Bump the Cargo
version first; the tag must match it exactly.
