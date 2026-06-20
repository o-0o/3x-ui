package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCloneInboundToNodeRequestBinding(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
	}{
		{
			name:        "json",
			contentType: "application/json",
			body:        `{"targetNodeId":23}`,
		},
		{
			name:        "form",
			contentType: "application/x-www-form-urlencoded; charset=UTF-8",
			body:        "targetNodeId=23",
		},
	}

	gin.SetMode(gin.TestMode)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			ctx.Request.Header.Set("Content-Type", tc.contentType)

			var req cloneInboundToNodeRequest
			if err := ctx.ShouldBind(&req); err != nil {
				t.Fatalf("ShouldBind: %v", err)
			}
			if req.TargetNodeID != 23 {
				t.Fatalf("TargetNodeID = %d, want 23", req.TargetNodeID)
			}
		})
	}
}
