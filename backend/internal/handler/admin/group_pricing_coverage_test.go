package admin

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupPricingCoverageEndpointPreviewsUnsavedPricing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	billing := service.NewBillingService(&config.Config{}, nil)
	coverage := service.NewGroupPricingCoverageService(service.NewModelPricingResolver(nil, billing))
	handler := NewGroupHandler(nil, nil, nil, coverage)
	router := gin.New()
	router.POST("/pricing-coverage", handler.PricingCoverage)

	body := []byte(`{
		"platform":"gemini",
		"models":["new-unique-model"],
		"model_pricing":[{
			"platform":"gemini",
			"models":["new-unique-model"],
			"billing_mode":"per_request",
			"per_request_price":0.04
		}]
	}`)
	request := httptest.NewRequest(http.MethodPost, "/pricing-coverage", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{
		"code":0,
		"message":"success",
		"data":{"models":[{
			"model":"new-unique-model",
			"status":"priced",
			"source":"group",
			"billing_mode":"per_request"
		}]}
	}`, recorder.Body.String())
}

func TestGroupPricingCoverageEndpointUsesExistingGroupPricing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminService := newStubAdminService()
	adminService.groups[0].ModelPricing = []service.ChannelModelPricing{{
		Platform:        service.PlatformAnthropic,
		Models:          []string{"existing-group-model"},
		BillingMode:     service.BillingModePerRequest,
		PerRequestPrice: coverageHandlerPrice(0.02),
	}}
	billing := service.NewBillingService(&config.Config{}, nil)
	coverage := service.NewGroupPricingCoverageService(service.NewModelPricingResolver(nil, billing))
	handler := NewGroupHandler(adminService, nil, nil, coverage)
	router := gin.New()
	router.POST("/pricing-coverage", handler.PricingCoverage)

	body := []byte(`{"group_id":2,"models":["existing-group-model"]}`)
	request := httptest.NewRequest(http.MethodPost, "/pricing-coverage", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"status":"priced"`)
	require.Contains(t, recorder.Body.String(), `"source":"group"`)
	require.Contains(t, recorder.Body.String(), `"billing_mode":"per_request"`)
}

func coverageHandlerPrice(value float64) *float64 { return &value }
