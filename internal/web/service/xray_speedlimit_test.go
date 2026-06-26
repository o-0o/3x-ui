package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestGetXrayConfigIncludesClientSpeedLimit(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()
	const email = "limited@example.com"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c007"
	const wantSpeed uint64 = 100 * 1024

	ib := &model.Inbound{
		UserId:         1,
		Tag:            "speed-vless",
		Enable:         true,
		Port:           21001,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[],"decryption":"none"}`,
		StreamSettings: `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	client := model.Client{Email: email, ID: uid, Enable: true, SpeedLimit: wantSpeed}
	if err := (&ClientService{}).SyncInbound(nil, ib.Id, []model.Client{client}); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}

	cfg, err := (&XrayService{}).GetXrayConfig()
	if err != nil {
		t.Fatalf("GetXrayConfig: %v", err)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	inbounds, _ := doc["inbounds"].([]any)
	if len(inbounds) == 0 {
		t.Fatalf("generated config has no inbounds: %s", raw)
	}
	var settings map[string]any
	for _, rawInbound := range inbounds {
		inbound, _ := rawInbound.(map[string]any)
		if inbound["tag"] == ib.Tag {
			settings, _ = inbound["settings"].(map[string]any)
			break
		}
	}
	if settings == nil {
		t.Fatalf("generated config missing inbound tag %q: %s", ib.Tag, raw)
	}
	clients, _ := settings["clients"].([]any)
	if len(clients) != 1 {
		t.Fatalf("generated config clients = %v, want one", settings["clients"])
	}
	got, _ := clients[0].(map[string]any)["speedLimit"].(float64)
	if uint64(got) != wantSpeed {
		t.Fatalf("generated speedLimit = %v, want %d; config=%s", got, wantSpeed, raw)
	}
}

func TestBuildRuntimeInboundForAPIEnrichesSpeedLimitFromClientRecord(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()
	const limitedEmail = "limited-node@example.com"
	const unlimitedEmail = "unlimited-node@example.com"
	const limitedID = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c007"
	const unlimitedID = "37b0217d-b51b-412d-9cca-3e2b90c6a721"
	const wantSpeed uint64 = 314573

	ib := &model.Inbound{
		UserId:   1,
		Tag:      "remote-speed-vless",
		Enable:   true,
		Port:     21002,
		Protocol: model.VLESS,
		Settings: `{
		  "clients": [
		    {"email":"limited-node@example.com","id":"ce8d33df-3a64-4f10-8f9b-91c3a8e0c007"},
		    {"email":"unlimited-node@example.com","id":"37b0217d-b51b-412d-9cca-3e2b90c6a721","speedLimit":12345}
		  ],
		  "decryption": "none"
		}`,
		StreamSettings: `{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`,
	}
	if err := db.Create(ib).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	clients := []model.Client{
		{Email: limitedEmail, ID: limitedID, Enable: true, SpeedLimit: wantSpeed},
		{Email: unlimitedEmail, ID: unlimitedID, Enable: true, SpeedLimit: 0},
	}
	if err := (&ClientService{}).SyncInbound(nil, ib.Id, clients); err != nil {
		t.Fatalf("SyncInbound: %v", err)
	}

	runtimeInbound, err := (&InboundService{}).buildRuntimeInboundForAPI(db, ib)
	if err != nil {
		t.Fatalf("buildRuntimeInboundForAPI: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(runtimeInbound.Settings), &settings); err != nil {
		t.Fatalf("unmarshal runtime settings: %v", err)
	}
	runtimeClients, _ := settings["clients"].([]any)
	if len(runtimeClients) != 2 {
		t.Fatalf("runtime clients = %v, want 2", settings["clients"])
	}

	byEmail := map[string]map[string]any{}
	for _, rawClient := range runtimeClients {
		c, _ := rawClient.(map[string]any)
		email, _ := c["email"].(string)
		byEmail[email] = c
	}
	got, _ := byEmail[limitedEmail]["speedLimit"].(float64)
	if uint64(got) != wantSpeed {
		t.Fatalf("limited speedLimit = %v, want %d; settings=%s", got, wantSpeed, runtimeInbound.Settings)
	}
	if _, exists := byEmail[unlimitedEmail]["speedLimit"]; exists {
		t.Fatalf("unlimited client kept stale speedLimit; settings=%s", runtimeInbound.Settings)
	}
}
