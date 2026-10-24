//go:build unit

package service

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type imageStudioPaymentRequiredRepo struct {
	AccountRepository
	accounts      map[int64]*Account
	setErrorCalls []int64
}

func (r *imageStudioPaymentRequiredRepo) SetError(_ context.Context, id int64, message string) error {
	r.setErrorCalls = append(r.setErrorCalls, id)
	if account := r.accounts[id]; account != nil {
		account.Status = StatusError
		account.Schedulable = false
		account.ErrorMessage = message
	}
	return nil
}

type imageStudioPaymentRequiredUpstream struct {
	HTTPUpstream
	t          *testing.T
	accountIDs []int64
}

func (u *imageStudioPaymentRequiredUpstream) Do(request *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.t.Helper()
	require.True(u.t, OpenAIImagesEndpointFromContext(request.Context()))
	u.accountIDs = append(u.accountIDs, accountID)
	status := http.StatusOK
	body := `{"data":[{"b64_json":"` + base64.StdEncoding.EncodeToString([]byte("image-bytes")) + `"}]}`
	if accountID == 41 {
		status = http.StatusPaymentRequired
		body = `{"error":{"message":"Not enough Anlas. Required 26, Available 2"}}`
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

type imageStudioPaymentRequiredGateway struct {
	imageStudioFailoverGatewayStub
	t       *testing.T
	gateway *OpenAIGatewayService
}

func (g *imageStudioPaymentRequiredGateway) ForwardImages(ctx context.Context, c *gin.Context, account *Account, body []byte, parsed *OpenAIImagesRequest, mappedModel string) (*OpenAIForwardResult, error) {
	g.t.Helper()
	require.True(g.t, OpenAIImagesEndpointFromContext(ctx))
	require.True(g.t, OpenAIImagesEndpointFromContext(c.Request.Context()), "Studio must bind the image endpoint marker to its Gin request")
	return g.gateway.ForwardImages(ctx, c, account, body, parsed, mappedModel)
}

func TestImageStudioGatewayPaymentRequiredFailsOverWithoutDisablingAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accounts := make(map[int64]*Account)
	for _, id := range []int64{41, 42} {
		accounts[id] = &Account{
			ID:          id,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{
				"api_key":  "test-image-key",
				"base_url": "https://images.example.test/v1",
			},
		}
	}
	repo := &imageStudioPaymentRequiredRepo{accounts: accounts}
	upstream := &imageStudioPaymentRequiredUpstream{t: t}
	forwarder := newOpenAIImagesTestService(upstream)
	forwarder.accountRepo = repo
	forwarder.rateLimitService = NewRateLimitService(repo, nil, forwarder.cfg, nil, nil)
	forwarder.rateLimitService.SetAccountRuntimeBlocker(forwarder)
	gateway := &imageStudioPaymentRequiredGateway{t: t, gateway: forwarder}
	gateway.selectAccount = func(excluded map[int64]struct{}) (*AccountSelectionResult, error) {
		for _, id := range []int64{41, 42} {
			account := accounts[id]
			if _, failed := excluded[id]; failed || !account.IsSchedulable() || forwarder.isOpenAIAccountRuntimeBlocked(account) {
				continue
			}
			return &AccountSelectionResult{Account: account, Acquired: true}, nil
		}
		return nil, ErrNoAvailableAccounts
	}
	groupID := int64(25)
	executor := NewImageStudioGatewayExecutor(&imageStudioAPIKeyProviderStub{apiKey: &APIKey{
		ID: 22, UserID: 7, GroupID: &groupID,
		User:  &User{ID: 7, Balance: 10},
		Group: &Group{ID: groupID, Platform: PlatformOpenAI, AllowImageGeneration: true},
	}}, nil, nil, gateway)

	for request := 0; request < 2; request++ {
		result, err := executor.Generate(context.Background(), ImageStudioGenerateInput{
			UserID: 7, Model: "gpt-image-1", Prompt: "a blue square",
		})
		require.NoError(t, err)
		require.Equal(t, []byte("image-bytes"), result.ImageBytes)
		require.Empty(t, repo.setErrorCalls, "an ordinary image billing rejection must not permanently disable its account")
		require.Equal(t, StatusActive, accounts[41].Status)
		require.True(t, accounts[41].IsSchedulable())
		require.False(t, forwarder.isOpenAIAccountRuntimeBlocked(accounts[41]))
	}
	require.Equal(t, []int64{41, 42, 41, 42}, upstream.accountIDs, "a later Studio request must still be allowed to try the first account")
}
