package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type lotteryAdminRepoStub struct {
	service.LotteryRepository
	draws      []service.LotteryAdminDraw
	lastOffset int
	lastLimit  int
}

func (s *lotteryAdminRepoStub) ListAdminDraws(_ context.Context, offset, limit int) ([]service.LotteryAdminDraw, int, error) {
	s.lastOffset = offset
	s.lastLimit = limit
	return s.draws, len(s.draws), nil
}

func TestLotteryAdminHandlerListsDrawRecordsWithPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &lotteryAdminRepoStub{draws: []service.LotteryAdminDraw{{
		ID: 7, UserID: 11, UserEmail: "winner@example.com", PrizeName: "余额奖励",
		PrizeType: service.LotteryPrizeTypeBalance, CreatedAt: time.Now(),
	}}}
	svc := service.NewLotteryService(nil, repo, nil, nil)
	h := NewLotteryHandler(svc)
	router := gin.New()
	router.GET("/admin/lottery/draws", h.ListDraws)

	req := httptest.NewRequest(http.MethodGet, "/admin/lottery/draws?page=2&page_size=10", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if repo.lastOffset != 10 || repo.lastLimit != 10 {
		t.Fatalf("pagination = (%d, %d), want (10, 10)", repo.lastOffset, repo.lastLimit)
	}
	var envelope struct {
		Data struct {
			Items []service.LotteryAdminDraw `json:"items"`
			Total int64                      `json:"total"`
			Page  int                        `json:"page"`
		} `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.Items) != 1 || envelope.Data.Items[0].UserEmail != "winner@example.com" || envelope.Data.Items[0].PrizeName != "余额奖励" {
		t.Fatalf("unexpected records: %#v", envelope.Data)
	}
	if envelope.Data.Total != 1 || envelope.Data.Page != 2 {
		t.Fatalf("unexpected pagination response: %#v", envelope.Data)
	}
}
