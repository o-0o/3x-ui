# Token Bucket Speed Limit Maintenance

This branch keeps the fork-specific code isolated so upstream `main` can be
merged with minimal conflict.

For the Chinese step-by-step install/update guide, see
`docs/tokenbucket-usage-zh.md`.

## Fork-specific areas

- `tools/xray-tokenbucket/`
  - Builds a patched Xray-core binary from upstream Xray-core `v26.6.22`.
  - Contains the Xray-core patch. Keep Xray changes here instead of vendoring a
    full Xray-core tree into this repository.
- `internal/database/model/model.go`
  - Adds `Client.SpeedLimit` and `ClientRecord.SpeedLimit`.
- `internal/web/service/xray.go`
  - Emits `speedLimit` into Xray users only when the value is positive.
- `frontend/src/pages/clients/*`
  - UI field for MB/s input. The database and Xray config use bytes per second.
- `install.sh`, `update.sh`, `x-ui.sh`
  - Point install/update commands at this fork and preserve `/etc/x-ui/`.

## Merging upstream main

```bash
git fetch upstream
git switch feature/xray-core-tokenbucket-speed-limit
git merge upstream/main
go test ./internal/database/model ./internal/web/service ./internal/web/runtime
npm --prefix frontend run build
```

Expected conflict hot spots are the client model, client form, generated schemas,
and release workflow. Keep the token-bucket changes small and re-run the tests
above after resolving conflicts.

## Updating an installed server without reinstalling

Use `update.sh`, not `install.sh`, for already configured panels. It replaces
`/usr/local/x-ui` and `/usr/bin/x-ui`, then restarts the service. It does not
change `/etc/x-ui/x-ui.db`, so the panel port, web path, credentials, inbounds,
clients, and node settings are preserved.

Latest release:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/o-0o/3x-ui/feature/xray-core-tokenbucket-speed-limit/update.sh)
```

Specific release:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/o-0o/3x-ui/feature/xray-core-tokenbucket-speed-limit/update.sh) v3.4.0-tokenbucket.2
```

Fresh install is only for a new server:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/o-0o/3x-ui/feature/xray-core-tokenbucket-speed-limit/install.sh)
```
