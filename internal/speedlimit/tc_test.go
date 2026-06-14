package speedlimit

import (
	"strings"
	"testing"
)

func TestParseRouteInterface(t *testing.T) {
	got, err := ParseRouteInterface("1.1.1.1 via 10.0.0.1 dev eth0 src 10.0.0.2 uid 1000")
	if err != nil {
		t.Fatal(err)
	}
	if got != "eth0" {
		t.Fatalf("interface = %q, want eth0", got)
	}
}

func TestNormalizeRulesUsesSmallestLimitPerPort(t *testing.T) {
	got := normalizeRules([]Rule{
		{Port: 30002, KBps: 500},
		{Port: 30001, KBps: 1000},
		{Port: 30001, KBps: 100},
		{Port: 0, KBps: 100},
		{Port: 30003, KBps: 0},
	})
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %#v", len(got), got)
	}
	if got[0].Port != 30001 || got[0].KBps != 100 {
		t.Fatalf("first rule = %#v, want port 30001 at 100 KB/s", got[0])
	}
	if got[1].Port != 30002 || got[1].KBps != 500 {
		t.Fatalf("second rule = %#v, want port 30002 at 500 KB/s", got[1])
	}
}

func TestReconcileInterfaceBuildsHTBFilters(t *testing.T) {
	oldStateDir := stateDir
	stateDir = t.TempDir()
	t.Cleanup(func() { stateDir = oldStateDir })

	var calls []string
	run := func(name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return nil
	}
	if err := ReconcileInterface("eth0", []Rule{{Port: 30173, KBps: 100}}, run); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(calls, "\n")
	for _, want := range []string{
		"tc qdisc replace dev eth0 root handle 30: htb default 1",
		"tc class replace dev eth0 parent 30: classid 30:100 htb rate 800kbit ceil 800kbit",
		"tc filter add dev eth0 protocol ip parent 30: prio 10 flower ip_proto tcp src_port 30173 classid 30:100",
		"tc filter add dev eth0 protocol ip parent 30: prio 10 flower ip_proto udp src_port 30173 classid 30:100",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing command %q in:\n%s", want, joined)
		}
	}
}

func TestReconcileConnectionsBuildsSharedEmailClass(t *testing.T) {
	oldStateDir := stateDir
	stateDir = t.TempDir()
	t.Cleanup(func() { stateDir = oldStateDir })

	var calls []string
	run := func(name string, args ...string) error {
		calls = append(calls, name+" "+strings.Join(args, " "))
		return nil
	}
	err := ReconcileConnections("eth0", []ConnectionRule{
		{Email: "alice", IP: "203.0.113.10", Port: 45678, Proto: "tcp", KBps: 100},
		{Email: "alice", IP: "203.0.113.10", Port: 45679, Proto: "tcp", KBps: 100},
	}, run)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(calls, "\n")
	for _, want := range []string{
		"tc qdisc replace dev eth0 root handle 31: htb default 1",
		"tc class replace dev eth0 parent 31: classid 31:100 htb rate 800kbit ceil 800kbit",
		"tc filter add dev eth0 protocol ip parent 31: prio 10 flower ip_proto tcp dst_ip 203.0.113.10 dst_port 45678 classid 31:100",
		"tc filter add dev eth0 protocol ip parent 31: prio 11 flower ip_proto tcp dst_ip 203.0.113.10 dst_port 45679 classid 31:100",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing command %q in:\n%s", want, joined)
		}
	}
}
