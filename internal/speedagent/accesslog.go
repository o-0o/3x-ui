package speedagent

import (
	"net"
	"regexp"
	"strconv"
	"strings"
)

var accessLineRE = regexp.MustCompile(`from (tcp|udp):(\[?[0-9a-fA-F:.]+\]?):([0-9]+) accepted .* email: (.+)$`)

type Observation struct {
	Email string
	IP    string
	Port  int
	Proto string
}

func ParseAccessLine(line string) (Observation, bool) {
	matches := accessLineRE.FindStringSubmatch(strings.TrimSpace(line))
	if len(matches) != 5 {
		return Observation{}, false
	}
	port, err := strconv.Atoi(matches[3])
	if err != nil || port <= 0 || port > 65535 {
		return Observation{}, false
	}
	ip := strings.Trim(matches[2], "[]")
	if net.ParseIP(ip) == nil {
		return Observation{}, false
	}
	email := strings.TrimSpace(matches[4])
	if email == "" {
		return Observation{}, false
	}
	return Observation{
		Email: email,
		IP:    ip,
		Port:  port,
		Proto: strings.ToLower(matches[1]),
	}, true
}
