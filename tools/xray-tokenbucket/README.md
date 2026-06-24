# Xray Token Bucket Speed Limit

This branch adds a per-client `speedLimit` field to 3x-ui and expects a matching
forked Xray-core binary that understands the same field.

The value is stored and emitted as bytes per second. The UI shows it as MB/s.
`0` means unlimited.

## Build the Xray binary locally

```bash
tools/xray-tokenbucket/build-xray.sh
```

The script clones Xray-core `v26.6.22`, applies
`patches/xray-core-v26.6.22-tokenbucket-speedlimit.patch`, builds linux amd64,
and writes:

```text
tools/xray-tokenbucket/dist/Xray-linux-64.zip
```

The 3x-ui release workflow does the same clone, patch, and build step inside
GitHub Actions, then packages the resulting `xray` binary directly into the
`x-ui-linux-amd64.tar.gz` release asset. No separate Xray-core fork release is
required.

## Runtime behavior

3x-ui emits users like:

```json
{
  "id": "...",
  "email": "user@example.com",
  "speedLimit": 1048576
}
```

The patched Xray-core converts that into `protocol.MemoryUser.SpeedLimit` and
wraps dispatcher uplink/downlink writers with shared token buckets keyed by
`email + direction`.

Both directions are capped by the same value. Multiple connections from the
same user share the same uplink bucket and the same downlink bucket, so opening
more connections does not multiply the user's cap.

See `docs/tokenbucket-maintenance.md` for upstream merge and preserve-config
update instructions.
