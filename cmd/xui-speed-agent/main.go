package main

import (
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/speedagent"
	"github.com/mhsanaei/3x-ui/v3/internal/speedlimit"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"

	_ "github.com/mattn/go-sqlite3"
)

type liveObservation struct {
	speedagent.Observation
	Seen time.Time
}

func main() {
	var (
		dbPath      = flag.String("db", config.GetDBPath(), "path to x-ui sqlite database")
		accessPath  = flag.String("access-log", "", "path to Xray access log; defaults to the configured Xray access log")
		dev         = flag.String("dev", "", "egress interface; defaults to route to 1.1.1.1")
		interval    = flag.Duration("interval", 2*time.Second, "tc reconcile interval")
		ttl         = flag.Duration("ttl", 90*time.Second, "active connection TTL after the last access-log observation")
		startAtEnd  = flag.Bool("start-at-end", true, "ignore existing access-log content on startup")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()
	if *showVersion {
		fmt.Println("xui-speed-agent experimental")
		return
	}

	if *accessPath == "" {
		path, err := xray.GetAccessLogPath()
		if err != nil {
			log.Fatalf("resolve access log: %v", err)
		}
		if path == "" || path == "none" {
			log.Fatal("Xray access log is disabled; configure log.access before running xui-speed-agent")
		}
		*accessPath = path
	}
	if *dev == "" {
		iface, err := speedlimit.DefaultInterface()
		if err != nil {
			log.Fatalf("detect egress interface: %v", err)
		}
		*dev = iface
	}

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lines := make(chan string, 256)
	go tailAccessLog(ctx, *accessPath, *startAtEnd, lines)

	observed := map[string]liveObservation{}
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	log.Printf("xui-speed-agent watching %s on %s, db=%s", *accessPath, *dev, *dbPath)
	for {
		select {
		case line := <-lines:
			if obs, ok := speedagent.ParseAccessLine(line); ok {
				key := obs.Email + "|" + obs.Proto + "|" + obs.IP + "|" + fmt.Sprint(obs.Port)
				observed[key] = liveObservation{Observation: obs, Seen: time.Now()}
			}
		case <-ticker.C:
			speeds, err := loadSpeeds(db)
			if err != nil {
				log.Printf("load speeds: %v", err)
				continue
			}
			rules := buildRules(observed, speeds, *ttl)
			if err := speedlimit.ReconcileConnections(*dev, rules, commandRunner); err != nil {
				log.Printf("reconcile tc: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func commandRunner(name string, args ...string) error {
	return speedlimit.Run(name, args...)
}

func loadSpeeds(db *sql.DB) (map[string]int, error) {
	rows, err := db.Query(`SELECT email, speed_limit FROM clients WHERE enable = 1 AND speed_limit > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var email string
		var speed int
		if err := rows.Scan(&email, &speed); err != nil {
			return nil, err
		}
		if email != "" && speed > 0 {
			out[email] = speed
		}
	}
	return out, rows.Err()
}

func buildRules(observed map[string]liveObservation, speeds map[string]int, ttl time.Duration) []speedlimit.ConnectionRule {
	now := time.Now()
	keys := make([]string, 0, len(observed))
	for key := range observed {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	rules := make([]speedlimit.ConnectionRule, 0, len(keys))
	for _, key := range keys {
		obs := observed[key]
		if now.Sub(obs.Seen) > ttl {
			delete(observed, key)
			continue
		}
		speed := speeds[obs.Email]
		if speed <= 0 {
			continue
		}
		rules = append(rules, speedlimit.ConnectionRule{
			Email: obs.Email,
			IP:    obs.IP,
			Port:  obs.Port,
			Proto: obs.Proto,
			KBps:  speed,
		})
	}
	return rules
}

func tailAccessLog(ctx context.Context, path string, startAtEnd bool, lines chan<- string) {
	var offset int64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		file, err := os.Open(path)
		if err != nil {
			log.Printf("open access log: %v", err)
			time.Sleep(time.Second)
			continue
		}
		if startAtEnd && offset == 0 {
			if pos, err := file.Seek(0, io.SeekEnd); err == nil {
				offset = pos
			}
		} else if offset > 0 {
			_, _ = file.Seek(offset, io.SeekStart)
		}

		reader := bufio.NewReader(file)
		for {
			line, err := reader.ReadString('\n')
			if line != "" {
				offset += int64(len(line))
				select {
				case lines <- line:
				default:
					log.Printf("access-log line dropped: channel full")
				}
			}
			if err == nil {
				continue
			}
			_ = file.Close()
			if stat, statErr := os.Stat(path); statErr == nil && stat.Size() < offset {
				offset = 0
			}
			time.Sleep(500 * time.Millisecond)
			break
		}
	}
}
