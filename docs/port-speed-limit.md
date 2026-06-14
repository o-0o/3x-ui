# Port Speed Limit

This branch implements real speed limits with Linux `tc`.

The limit is stored on the client as `speedLimit` in KB/s, but enforcement is
done on the inbound port. This is intentional: Linux can reliably shape packets
by port without changing Xray core.

## Behavior

- `0` means unlimited.
- A positive value limits egress traffic whose source port is the inbound port.
- If multiple clients share one inbound, they share the same port limit.
- If multiple clients on the same inbound have different positive limits, the
  smallest value is used for that inbound port.
- Remote nodes must run this branch too. The master panel stores the value and
  sends it to nodes, but only the node can install local `tc` rules.

## Requirements

- Linux node
- root service privileges
- `iproute2` / `tc`
- kernel support for `sch_htb`, `cls_flower`, and preferably `sch_fq_codel`

Debian/Ubuntu usually already has `iproute2`. If `tc` modules are missing:

```bash
apt update
apt install -y iproute2 linux-modules-extra-$(uname -r)
```

## Verify

After setting a client speed limit and restarting Xray from the panel:

```bash
tc -s qdisc show
tc filter show dev "$(ip route get 1.1.1.1 | awk '{for(i=1;i<=NF;i++) if($i=="dev"){print $(i+1); exit}}')" parent 30:
```

For a 100 KB/s limit, downloads through that inbound should stay near
800 kbit/s plus protocol overhead.
