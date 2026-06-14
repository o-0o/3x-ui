package speedlimit

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const (
	rootHandle = "30:"
	defaultID  = "30:1"
	stateFile  = "port-speedlimit.state"

	connectionRootHandle = "31:"
	connectionDefaultID  = "31:1"
	connectionStateFile  = "user-connection-speedlimit.state"
)

var stateDir = "/var/lib/3x-ui"

// Rule caps egress traffic whose source port is Port. KBps is kilobytes per
// second; 0 or lower is ignored.
type Rule struct {
	Port    int
	KBps    int
	Comment string
}

// ConnectionRule caps one observed client connection and assigns it to the
// email's shared class, so all active connections for the same email share the
// same speed.
type ConnectionRule struct {
	Email string
	IP    string
	Port  int
	Proto string
	KBps  int
}

// Runner executes a system command. Tests provide a fake runner.
type Runner func(name string, args ...string) error

func commandRunner(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Run(name string, args ...string) error {
	return commandRunner(name, args...)
}

// Reconcile applies port speed-limit rules to the default egress interface.
func Reconcile(rules []Rule) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	dev, err := DefaultInterface()
	if err != nil {
		return err
	}
	return ReconcileInterface(dev, rules, commandRunner)
}

// DefaultInterface returns the interface used for ordinary internet egress.
func DefaultInterface() (string, error) {
	out, err := exec.Command("ip", "route", "get", "1.1.1.1").Output()
	if err != nil {
		return "", err
	}
	return ParseRouteInterface(string(out))
}

func ParseRouteInterface(route string) (string, error) {
	fields := strings.Fields(route)
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "dev" {
			return fields[i+1], nil
		}
	}
	return "", errors.New("default route interface not found")
}

// ReconcileInterface owns the root qdisc while speed-limit rules are active.
// When the last rule is removed, it removes only qdiscs created by this package
// as tracked by the state file.
func ReconcileInterface(dev string, rules []Rule, run Runner) error {
	normalized := normalizeRules(rules)
	if len(normalized) == 0 {
		if !hasState(dev) {
			return nil
		}
		_ = run("tc", "qdisc", "del", "dev", dev, "root")
		removeState(dev)
		return nil
	}

	if err := run("modprobe", "sch_htb"); err != nil {
		return err
	}
	_ = run("modprobe", "cls_flower")
	_ = run("modprobe", "sch_fq_codel")

	if err := run("tc", "qdisc", "replace", "dev", dev, "root", "handle", rootHandle, "htb", "default", "1"); err != nil {
		return err
	}
	if err := run("tc", "class", "replace", "dev", dev, "parent", rootHandle, "classid", defaultID, "htb", "rate", "10gbit", "ceil", "10gbit"); err != nil {
		return err
	}

	for i, rule := range normalized {
		minor := strconv.Itoa(100 + i)
		classID := "30:" + minor
		rate := strconv.Itoa(rule.KBps*8) + "kbit"
		if err := run("tc", "class", "replace", "dev", dev, "parent", rootHandle, "classid", classID, "htb", "rate", rate, "ceil", rate); err != nil {
			return err
		}
		_ = run("tc", "qdisc", "replace", "dev", dev, "parent", classID, "fq_codel")
		port := strconv.Itoa(rule.Port)
		prio := strconv.Itoa(10 + i)
		for _, proto := range []string{"tcp", "udp"} {
			if err := run("tc", "filter", "add", "dev", dev, "protocol", "ip", "parent", rootHandle, "prio", prio, "flower", "ip_proto", proto, "src_port", port, "classid", classID); err != nil {
				return err
			}
			_ = run("tc", "filter", "add", "dev", dev, "protocol", "ipv6", "parent", rootHandle, "prio", prio, "flower", "ip_proto", proto, "src_port", port, "classid", classID)
		}
	}

	return writeState(dev)
}

// ReconcileConnections owns a separate root qdisc for user-level connection
// shaping. It is intended for the xui-speed-agent branch, where access.log maps
// a live client IP:port to an email.
func ReconcileConnections(dev string, rules []ConnectionRule, run Runner) error {
	normalized := normalizeConnectionRules(rules)
	if len(normalized) == 0 {
		if !hasNamedState(dev, connectionStateFile) {
			return nil
		}
		_ = run("tc", "qdisc", "del", "dev", dev, "root")
		removeNamedState(dev, connectionStateFile)
		return nil
	}

	if err := run("modprobe", "sch_htb"); err != nil {
		return err
	}
	_ = run("modprobe", "cls_flower")
	_ = run("modprobe", "sch_fq_codel")

	if err := run("tc", "qdisc", "replace", "dev", dev, "root", "handle", connectionRootHandle, "htb", "default", "1"); err != nil {
		return err
	}
	if err := run("tc", "class", "replace", "dev", dev, "parent", connectionRootHandle, "classid", connectionDefaultID, "htb", "rate", "10gbit", "ceil", "10gbit"); err != nil {
		return err
	}

	classByEmail := map[string]string{}
	emails, speedByEmail := uniqueLimitedEmails(normalized)
	for i, email := range emails {
		classID := "31:" + strconv.Itoa(100+i)
		classByEmail[email] = classID
		rate := strconv.Itoa(speedByEmail[email]*8) + "kbit"
		if err := run("tc", "class", "replace", "dev", dev, "parent", connectionRootHandle, "classid", classID, "htb", "rate", rate, "ceil", rate); err != nil {
			return err
		}
		_ = run("tc", "qdisc", "replace", "dev", dev, "parent", classID, "fq_codel")
	}

	i := 0
	for _, rule := range sortedConnectionRules(normalized) {
		classID := classByEmail[rule.Email]
		protocol := "ip"
		dstKey := "dst_ip"
		if strings.Contains(rule.IP, ":") {
			protocol = "ipv6"
		}
		prio := strconv.Itoa(10 + i)
		if err := run("tc", "filter", "add", "dev", dev, "protocol", protocol, "parent", connectionRootHandle, "prio", prio, "flower", "ip_proto", rule.Proto, dstKey, rule.IP, "dst_port", strconv.Itoa(rule.Port), "classid", classID); err != nil {
			return err
		}
		i++
	}
	return writeNamedState(dev, connectionStateFile)
}

func normalizeRules(rules []Rule) []Rule {
	byPort := map[int]Rule{}
	for _, rule := range rules {
		if rule.Port <= 0 || rule.Port > 65535 || rule.KBps <= 0 {
			continue
		}
		if old, ok := byPort[rule.Port]; !ok || rule.KBps < old.KBps {
			byPort[rule.Port] = rule
		}
	}
	ports := make([]int, 0, len(byPort))
	for port := range byPort {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	out := make([]Rule, 0, len(ports))
	for _, port := range ports {
		out = append(out, byPort[port])
	}
	return out
}

func normalizeConnectionRules(rules []ConnectionRule) map[string]ConnectionRule {
	out := map[string]ConnectionRule{}
	for _, rule := range rules {
		rule.Email = strings.TrimSpace(rule.Email)
		rule.Proto = strings.ToLower(strings.TrimSpace(rule.Proto))
		if rule.Email == "" || rule.IP == "" || rule.Port <= 0 || rule.Port > 65535 || rule.KBps <= 0 {
			continue
		}
		if rule.Proto != "tcp" && rule.Proto != "udp" {
			continue
		}
		key := rule.Email + "|" + rule.Proto + "|" + rule.IP + "|" + strconv.Itoa(rule.Port)
		out[key] = rule
	}
	return out
}

func uniqueLimitedEmails(rules map[string]ConnectionRule) ([]string, map[string]int) {
	byEmail := map[string]ConnectionRule{}
	for _, rule := range rules {
		if old, ok := byEmail[rule.Email]; !ok || rule.KBps < old.KBps {
			byEmail[rule.Email] = rule
		}
	}
	emails := make([]string, 0, len(byEmail))
	for email := range byEmail {
		emails = append(emails, email)
	}
	sort.Strings(emails)
	for _, email := range emails {
		for key, rule := range rules {
			if rule.Email == email {
				rule.KBps = byEmail[email].KBps
				rules[key] = rule
			}
		}
	}
	speeds := make(map[string]int, len(byEmail))
	for email, rule := range byEmail {
		speeds[email] = rule.KBps
	}
	return emails, speeds
}

func sortedConnectionRules(rules map[string]ConnectionRule) []ConnectionRule {
	keys := make([]string, 0, len(rules))
	for key := range rules {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]ConnectionRule, 0, len(keys))
	for _, key := range keys {
		out = append(out, rules[key])
	}
	return out
}

func statePath(dev string) string {
	return namedStatePath(dev, stateFile)
}

func namedStatePath(dev, file string) string {
	clean := strings.NewReplacer("/", "_", " ", "_").Replace(dev)
	return filepath.Join(stateDir, clean+"-"+file)
}

func hasState(dev string) bool {
	return hasNamedState(dev, stateFile)
}

func hasNamedState(dev, file string) bool {
	_, err := os.Stat(namedStatePath(dev, file))
	return err == nil
}

func writeState(dev string) error {
	return writeNamedState(dev, stateFile)
}

func writeNamedState(dev, file string) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(namedStatePath(dev, file), []byte(dev+"\n"), 0o644)
}

func removeState(dev string) {
	removeNamedState(dev, stateFile)
}

func removeNamedState(dev, file string) {
	_ = os.Remove(namedStatePath(dev, file))
}
