package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type lotteryAdminRepoStub struct {
	service.LotteryRepository
	draws     []service.LotteryAdminDraw
	lastQuery service.LotteryAdminDrawQuery
}

type lotteryAttemptGrantRepoStub struct {
	service.LotteryRepository
	input        service.LotteryAttemptGrantInput
	result       service.LotteryAttemptGrantResult
	previewInput service.LotteryAttemptGrantInput
	preview      service.LotteryAttemptGrantPreviewResult
}

type lotteryAttemptBalanceRepoStub struct {
	service.LotteryRepository
	activity *service.LotteryActivity
	query    service.LotteryAdminAttemptBalanceQuery
	rows     []service.LotteryAdminAttemptBalance
	total    int
}

func (s *lotteryAttemptBalanceRepoStub) GetActiveActivity(context.Context, time.Time, bool) (*service.LotteryActivity, error) {
	return s.activity, nil
}

func (s *lotteryAttemptBalanceRepoStub) ListAdminAttemptBalances(_ context.Context, query service.LotteryAdminAttemptBalanceQuery) ([]service.LotteryAdminAttemptBalance, int, error) {
	s.query = query
	return s.rows, s.total, nil
}

func (s *lotteryAttemptGrantRepoStub) GrantLotteryAttempts(_ context.Context, input service.LotteryAttemptGrantInput) (service.LotteryAttemptGrantResult, error) {
	s.input = input
	return s.result, nil
}

func (s *lotteryAttemptGrantRepoStub) PreviewLotteryAttemptGrant(_ context.Context, input service.LotteryAttemptGrantInput) (service.LotteryAttemptGrantPreviewResult, error) {
	s.previewInput = input
	return s.preview, nil
}

func (s *lotteryAdminRepoStub) ListAdminDraws(_ context.Context, query service.LotteryAdminDrawQuery) ([]service.LotteryAdminDraw, int, error) {
	s.lastQuery = query
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

	req := httptest.NewRequest(http.MethodGet, "/admin/lottery/draws?page=2&page_size=10&user_id=11&prize_type=product&attempt_source=wallet&winners_only=true", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	wantQuery := service.LotteryAdminDrawQuery{
		Offset: 10, Limit: 10, UserID: 11, PrizeType: service.LotteryPrizeTypeProduct,
		AttemptSource: service.LotteryAttemptSourceWallet, WinnersOnly: true,
	}
	if repo.lastQuery != wantQuery {
		t.Fatalf("query = %#v, want %#v", repo.lastQuery, wantQuery)
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

func TestLotteryAdminHandlerRejectsInvalidDrawFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &lotteryAdminRepoStub{}
	svc := service.NewLotteryService(nil, repo, nil, nil)
	h := NewLotteryHandler(svc)
	router := gin.New()
	router.GET("/admin/lottery/draws", h.ListDraws)

	for _, rawQuery := range []string{
		"prize_type=invalid",
		"attempt_source=invalid",
		"user_id=abc",
		"winners_only=maybe",
	} {
		req := httptest.NewRequest(http.MethodGet, "/admin/lottery/draws?"+rawQuery, nil)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("query %q status = %d, body = %s", rawQuery, res.Code, res.Body.String())
		}
	}
}

func TestLotteryAdminHandlerGrantsAttemptsWithAuthenticatedAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &lotteryAttemptGrantRepoStub{result: service.LotteryAttemptGrantResult{Affected: 2, TotalGranted: 6}}
	svc := service.NewLotteryService(nil, repo, nil, nil)
	h := NewLotteryHandler(svc)
	router := gin.New()
	router.POST("/admin/lottery/attempts/grant", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
		c.Next()
	}, h.GrantAttempts)

	req := httptest.NewRequest(http.MethodPost, "/admin/lottery/attempts/grant", bytes.NewBufferString(`{"user_ids":[11,12],"amount":3,"description":"manual bonus","request_key":"handler-request-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "handler-request-1")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if repo.input.CreatedBy != 99 || len(repo.input.UserIDs) != 2 || repo.input.Amount != 3 || repo.input.RequestKey != "handler-request-1" {
		t.Fatalf("unexpected service input: %#v", repo.input)
	}
	var envelope struct {
		Data service.LotteryAttemptGrantResult `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data != repo.result {
		t.Fatalf("unexpected grant result: %#v", envelope.Data)
	}
}

func TestLotteryAdminHandlerGrantsAttemptsToActiveUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &lotteryAttemptGrantRepoStub{result: service.LotteryAttemptGrantResult{Affected: 4, TotalGranted: 8}}
	svc := service.NewLotteryService(nil, repo, nil, nil)
	h := NewLotteryHandler(svc)
	router := gin.New()
	router.POST("/admin/lottery/attempts/grant", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
		c.Next()
	}, h.GrantAttempts)

	req := httptest.NewRequest(http.MethodPost, "/admin/lottery/attempts/grant", bytes.NewBufferString(`{"target":"active","active_days":30,"amount":2,"request_key":"active-handler-request-1"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "active-handler-request-1")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if repo.input.Target != service.LotteryAttemptGrantTargetActive || repo.input.ActiveDays != service.LotteryAttemptActiveDays30 || repo.input.CreatedBy != 99 {
		t.Fatalf("unexpected active grant input: %#v", repo.input)
	}
}

func TestLotteryAdminHandlerPreviewsActiveAttemptTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &lotteryAttemptGrantRepoStub{preview: service.LotteryAttemptGrantPreviewResult{Count: 12}}
	svc := service.NewLotteryService(nil, repo, nil, nil)
	h := NewLotteryHandler(svc)
	router := gin.New()
	router.POST("/admin/lottery/attempts/preview", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
		c.Next()
	}, h.PreviewAttempts)

	req := httptest.NewRequest(http.MethodPost, "/admin/lottery/attempts/preview", bytes.NewBufferString(`{"target":"active","active_days":7}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if repo.previewInput.Target != service.LotteryAttemptGrantTargetActive || repo.previewInput.ActiveDays != service.LotteryAttemptActiveDays7 {
		t.Fatalf("unexpected preview input: %#v", repo.previewInput)
	}
	var envelope struct {
		Data service.LotteryAttemptGrantPreviewResult `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Count != 12 {
		t.Fatalf("unexpected preview response: %#v", envelope.Data)
	}
}

func TestLotteryAdminHandlerRejectsUnsupportedActiveWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &lotteryAttemptGrantRepoStub{}
	svc := service.NewLotteryService(nil, repo, nil, nil)
	h := NewLotteryHandler(svc)
	router := gin.New()
	router.POST("/admin/lottery/attempts/preview", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
		c.Next()
	}, h.PreviewAttempts)

	req := httptest.NewRequest(http.MethodPost, "/admin/lottery/attempts/preview", bytes.NewBufferString(`{"target":"active","active_days":14}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
}

func TestLotteryAdminHandlerRejectsInvalidAttemptGrant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &lotteryAttemptGrantRepoStub{}
	svc := service.NewLotteryService(nil, repo, nil, nil)
	h := NewLotteryHandler(svc)
	router := gin.New()
	router.POST("/admin/lottery/attempts/grant", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
		c.Next()
	}, h.GrantAttempts)

	req := httptest.NewRequest(http.MethodPost, "/admin/lottery/attempts/grant", bytes.NewBufferString(`{"amount":3}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
}

func TestLotteryAdminHandlerListsAttemptBalancesWithPaginationAndSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &lotteryAttemptBalanceRepoStub{
		activity: &service.LotteryActivity{ID: 5, AttemptMode: service.LotteryAttemptModeTotal, AttemptLimit: 3},
		rows:     []service.LotteryAdminAttemptBalance{{UserID: 11, UserEmail: "alice@example.com", TotalRemaining: 3}},
		total:    1,
	}
	svc := service.NewLotteryService(nil, repo, nil, nil)
	h := NewLotteryHandler(svc)
	router := gin.New()
	router.GET("/admin/lottery/attempts", h.ListAttemptBalances)

	req := httptest.NewRequest(http.MethodGet, "/admin/lottery/attempts?page=2&page_size=10&search=alice", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if repo.query.Offset != 10 || repo.query.Limit != 10 || repo.query.Search != "alice" {
		t.Fatalf("query = %#v", repo.query)
	}
	var envelope struct {
		Data struct {
			Items []service.LotteryAdminAttemptBalance `json:"items"`
			Total int64                                `json:"total"`
			Page  int                                  `json:"page"`
		} `json:"data"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.Items) != 1 || envelope.Data.Items[0].UserEmail != "alice@example.com" || envelope.Data.Total != 1 || envelope.Data.Page != 2 {
		t.Fatalf("unexpected response: %#v", envelope.Data)
	}
}
