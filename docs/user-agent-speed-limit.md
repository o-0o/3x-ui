# User Agent Speed Limit

This branch implements an experimental user-level limiter for the "one inbound,
many clients" layout.

It does not use Xray `policy.levels` for bandwidth control. Instead, a node-side
agent watches Xray access logs, maps live client connections to emails, and
installs Linux `tc flower` filters for those connections.

## How It Works

1. The panel stores `clients.speed_limit` in KB/s.
2. `xui-speed-agent` reads the local SQLite database every few seconds.
3. The agent tails the Xray access log and parses lines like:

   ```text
   from tcp:203.0.113.10:2387 accepted tcp:example.com:443 email: alice
   ```

4. For each live limited email, it creates one HTB class.
5. Each observed connection for that email is filtered by destination
   `IP:port` and assigned to that email's shared class.
6. The agent only rebuilds `tc` rules when the live rule set changes, avoiding
   repeated `tc` churn while connections are stable.

The result is real per-email downstream shaping for active TCP/UDP connections,
even when multiple users share one inbound port.

## Requirements

- Linux node
- root privileges
- `iproute2` / `tc`
- kernel support for `sch_htb`, `cls_flower`, and preferably `sch_fq_codel`
- Xray access log enabled
- SQLite database on the node

## Build

```bash
go build -o xui-speed-agent ./cmd/xui-speed-agent
```

## Run

```bash
./xui-speed-agent
```

Useful flags:

```bash
./xui-speed-agent \
  -db /etc/x-ui/x-ui.db \
  -access-log /usr/local/x-ui/access.log \
  -dev eth0 \
  -interval 2s \
  -ttl 90s
```

## Verify

```bash
tc -s qdisc show
tc filter show dev eth0 parent 31:
```

Set a client to `100` KB/s and start a fresh connection. That user's download
traffic should stay near 800 kbit/s plus protocol overhead.

## Limits

- The limiter acts after a connection appears in the access log, so the first
  packets of a new connection may pass before shaping is installed.
- If access logs are disabled or delayed, the agent cannot identify users.
- It shapes downstream traffic from node to client. Upload shaping needs a
  separate ingress/IFB path.
- Rule changes rebuild the managed root qdisc so stale per-connection filters
  do not keep applying old limits.
- This branch is intentionally separate from the simpler port-based branch.
