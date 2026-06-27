package service

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestSyncInboundUpdatesExistingClientSpeedLimit(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()
	inbound := &model.Inbound{
		Tag:            "speed-limit-sync",
		Enable:         true,
		Port:           49001,
		Protocol:       model.VLESS,
		StreamSettings: `{"network":"tcp","security":"none"}`,
		Settings:       `{"decryption":"none","clients":[]}`,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	client := model.Client{
		ID:         "11111111-1111-4111-8111-111111111111",
		Email:      "limited@example.com",
		Enable:     true,
		SpeedLimit: 100 * 1024,
	}
	service := &ClientService{}
	if err := service.SyncInbound(db, inbound.Id, []model.Client{client}); err != nil {
		t.Fatalf("initial SyncInbound: %v", err)
	}

	client.SpeedLimit = 300 * 1024
	if err := service.SyncInbound(db, inbound.Id, []model.Client{client}); err != nil {
		t.Fatalf("updated SyncInbound: %v", err)
	}

	var record model.ClientRecord
	if err := db.Where("email = ?", client.Email).First(&record).Error; err != nil {
		t.Fatalf("load client record: %v", err)
	}
	if record.SpeedLimit != client.SpeedLimit {
		t.Fatalf("stored speed limit = %d, want %d", record.SpeedLimit, client.SpeedLimit)
	}

	cfg, err := (&XrayService{}).GetXrayConfig()
	if err != nil {
		t.Fatalf("GetXrayConfig: %v", err)
	}
	var settings map[string]any
	for i := range cfg.InboundConfigs {
		if cfg.InboundConfigs[i].Tag != inbound.Tag {
			continue
		}
		if err := json.Unmarshal(cfg.InboundConfigs[i].Settings, &settings); err != nil {
			t.Fatalf("decode Xray inbound settings: %v", err)
		}
		break
	}
	if settings == nil {
		t.Fatalf("generated Xray config has no inbound %q", inbound.Tag)
	}
	clients := settings["clients"].([]any)
	got := uint64(clients[0].(map[string]any)["speedLimit"].(float64))
	if got != client.SpeedLimit {
		t.Fatalf("generated speed limit = %d, want %d", got, client.SpeedLimit)
	}
}
