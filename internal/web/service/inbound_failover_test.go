package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestPrepareFailoverClonePreservesLinkAndClients(t *testing.T) {
	sourceNode := 1
	source := &model.Inbound{
		Id:                   42,
		NodeID:               &sourceNode,
		OriginNodeGuid:       "source-guid",
		Tag:                  "n1-in-443-tcp",
		Remark:               "primary",
		ShareAddrStrategy:    "custom",
		ShareAddr:            "edge.example.com",
		Settings:             `{"clients":[{"email":"alice","id":"uuid"}]}`,
		Up:                   10,
		Down:                 20,
		LastTrafficResetTime: 30,
		ClientStats:          []xray.ClientTraffic{{Email: "alice"}},
	}

	clone := prepareFailoverClone(source, 2)
	if clone.Id != 0 || clone.NodeID == nil || *clone.NodeID != 2 {
		t.Fatalf("clone identity = id:%d node:%v", clone.Id, clone.NodeID)
	}
	if clone.Tag != "" || clone.OriginNodeGuid != "" {
		t.Fatalf("clone retained node identity: tag=%q origin=%q", clone.Tag, clone.OriginNodeGuid)
	}
	if clone.ShareAddrStrategy != "custom" || clone.ShareAddr != "edge.example.com" {
		t.Fatalf("stable share address was not preserved: %q %q", clone.ShareAddrStrategy, clone.ShareAddr)
	}
	if clone.Settings != source.Settings {
		t.Fatal("client settings were not preserved")
	}
	if clone.Up != 0 || clone.Down != 0 || clone.LastTrafficResetTime != 0 || clone.ClientStats != nil {
		t.Fatal("node-local counters were not reset")
	}
}
