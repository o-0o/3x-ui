package speedagent

import "testing"

func TestParseAccessLineIPv4(t *testing.T) {
	line := "2026/06/02 13:35:53 from tcp:203.0.113.10:2387 accepted tcp:example.com:443 email: alice"
	got, ok := ParseAccessLine(line)
	if !ok {
		t.Fatal("line was not parsed")
	}
	if got.Email != "alice" || got.IP != "203.0.113.10" || got.Port != 2387 || got.Proto != "tcp" {
		t.Fatalf("unexpected observation: %#v", got)
	}
}

func TestParseAccessLineIPv6(t *testing.T) {
	line := "2026/06/02 13:35:53 from udp:[2001:db8::1]:2387 accepted udp:example.com:443 email: bob"
	got, ok := ParseAccessLine(line)
	if !ok {
		t.Fatal("line was not parsed")
	}
	if got.Email != "bob" || got.IP != "2001:db8::1" || got.Port != 2387 || got.Proto != "udp" {
		t.Fatalf("unexpected observation: %#v", got)
	}
}
