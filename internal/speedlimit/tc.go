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
)

var stateDir = "/var/lib/3x-ui"

// Rule caps egress traffic whose source port is Port. KBps is kilobytes per
// second; 0 or lower is ignored.
type Rule struct {
	Port    int
	KBps    int
	Comment string
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

func statePath(dev string) string {
	clean := strings.NewReplacer("/", "_", " ", "_").Replace(dev)
	return filepath.Join(stateDir, clean+"-"+stateFile)
}

func hasState(dev string) bool {
	_, err := os.Stat(statePath(dev))
	return err == nil
}

func writeState(dev string) error {
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(statePath(dev), []byte(dev+"\n"), 0o644)
}

func removeState(dev string) {
	_ = os.Remove(statePath(dev))
}
