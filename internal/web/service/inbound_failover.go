package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// CloneInboundToNode prepares an already configured inbound on a backup node.
// The source remains online, allowing DNS to be switched only after the target
// has been tested. Client credentials and the custom share address are kept so
// existing client links remain valid.
func (s *InboundService) CloneInboundToNode(ctx context.Context, id, targetNodeID int) (*model.Inbound, error) {
	if targetNodeID <= 0 {
		return nil, fmt.Errorf("target node is required")
	}

	source, err := s.GetInboundDetail(id)
	if err != nil {
		return nil, err
	}
	if source.NodeID != nil && *source.NodeID == targetNodeID {
		return nil, fmt.Errorf("source and target nodes are the same")
	}
	if source.ShareAddrStrategy != "custom" || strings.TrimSpace(source.ShareAddr) == "" {
		return nil, fmt.Errorf("set a custom share address before preparing failover")
	}

	node, err := (&NodeService{}).GetById(targetNodeID)
	if err != nil {
		return nil, err
	}
	if !node.Enable || node.Status != "online" {
		return nil, fmt.Errorf("target node is not online")
	}

	clone := prepareFailoverClone(source, targetNodeID)
	created, _, err := s.AddInbound(clone)
	if err != nil {
		return nil, err
	}

	// AddInbound pushes the complete inbound, including all clients, to an
	// online node. Apply it immediately instead of waiting for the node's
	// periodic restart window.
	rt, rtErr := s.runtimeFor(created)
	if rtErr == nil {
		rtErr = rt.RestartXray(ctx)
	}
	if rtErr != nil {
		logger.Warning("failover clone: target xray restart failed; queued for reconcile:", rtErr)
		if dirtyErr := (&NodeService{}).MarkNodeDirty(targetNodeID); dirtyErr != nil {
			logger.Warning("failover clone: mark target node dirty failed:", dirtyErr)
		}
	}
	return created, nil
}

func prepareFailoverClone(source *model.Inbound, targetNodeID int) *model.Inbound {
	clone := *source
	clone.Id = 0
	clone.NodeID = &targetNodeID
	clone.OriginNodeGuid = ""
	clone.Tag = ""
	clone.Up = 0
	clone.Down = 0
	clone.LastTrafficResetTime = 0
	clone.ClientStats = nil
	clone.Remark = strings.TrimSpace(source.Remark) + " (backup)"
	return &clone
}
