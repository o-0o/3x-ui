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
