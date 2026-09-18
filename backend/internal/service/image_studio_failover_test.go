//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type imageStudioFailoverGatewayStub struct {
	imageStudioGatewayStub
	selectAccount func(map[int64]struct{}) (*AccountSelectionResult, error)
	forward       func(*gin.Context, *Account) (*OpenAIForwardResult, error)
}

func (s *imageStudioFailoverGatewayStub) SelectAccountWithSchedulerForImages(ctx context.Context, groupID *int64, sessionHash, model string, excluded map[int64]struct{}, capability OpenAIImagesCapability) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	selection, err := s.selectAccount(excluded)
	return selection, OpenAIAccountScheduleDecision{}, err
}

func (s *imageStudioFailoverGatewayStub) ForwardImages(ctx context.Context, c *gin.Context, account *Account, body []byte, parsed *OpenAIImagesRequest, model string) (*OpenAIForwardResult, error) {
	if s.forward != nil {
		if result, err := s.forward(c, account); result != nil || err != nil {
			return result, err
		}
	}
	return s.imageStudioGatewayStub.ForwardImages(ctx, c, account, body, parsed, model)
}

func newImageStudioFailoverTestExecutor(gateway ImageStudioGateway) *ImageStudioGatewayExecutor {
	groupID := int64(25)
	return NewImageStudioGatewayExecutor(&imageStudioAPIKeyProviderStub{apiKey: &APIKey{
		ID: 22, UserID: 7, GroupID: &groupID,
		User:  &User{ID: 7, Balance: 10},
		Group: &Group{ID: groupID, Platform: PlatformOpenAI, AllowImageGeneration: true},
	}}, nil, nil, gateway)
}

func TestImageStudioGatewayFailoverReleasesFailedAccountAndBillsOnlySuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var selected, released int
	var firstContext *gin.Context
	gateway := &imageStudioFailoverGatewayStub{}
	gateway.selectAccount = func(excluded map[int64]struct{}) (*AccountSelectionResult, error) {
		require.Equal(t, selected, released, "release the previous slot before selecting again")
		if selected > 0 {
			require.Contains(t, excluded, int64(1))
		}
		selected++
		return &AccountSelectionResult{
			Account: &Account{ID: int64(selected), Platform: PlatformOpenAI}, Acquired: true,
			ReleaseFunc: func() { released++ },
		}, nil
	}
	gateway.forward = func(c *gin.Context, account *Account) (*OpenAIForwardResult, error) {
		if account.ID == 1 {
			firstContext = c
			c.Set("failed_attempt", true)
			c.Header("X-Failed-Attempt", "true")
			return nil, &UpstreamFailoverError{StatusCode: http.StatusServiceUnavailable}
		}
		require.NotSame(t, firstContext, c)
		_, leaked := c.Get("failed_attempt")
		require.False(t, leaked)
		require.Empty(t, c.Writer.Header().Get("X-Failed-Attempt"))
		return nil, nil
	}
	result, err := newImageStudioFailoverTestExecutor(gateway).Generate(context.Background(), ImageStudioGenerateInput{
		UserID: 7, Model: "gpt-image-2", Prompt: "a blue square",
	})
	require.NoError(t, err)
	require.Equal(t, 2, selected)
	require.Equal(t, selected, released)
	require.Equal(t, []byte("image-bytes"), result.ImageBytes)
	require.Zero(t, gateway.recordCalls)
	require.NoError(t, result.CommitUsage(context.Background()))
	require.Equal(t, 1, gateway.recordCalls)
	require.Equal(t, int64(2), gateway.usageInput.Account.ID)
}

func TestImageStudioGatewayFailoverStopsSafely(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		partial    bool
		write      bool
		cancel     bool
		exhausted  bool
		wantSelect int
	}{
		{name: "permanent rejection", err: &OpenAIImagesUpstreamError{StatusCode: 400, Code: "content_policy_violation"}, wantSelect: 1},
		{name: "partial image", err: &UpstreamFailoverError{StatusCode: 503}, partial: true, wantSelect: 1},
		{name: "response already written", err: &UpstreamFailoverError{StatusCode: 503}, write: true, wantSelect: 1},
		{name: "explicit stop", err: &UpstreamFailoverError{StatusCode: 503, NextAccountAction: NextAccountStop}, wantSelect: 1},
		{name: "canceled", err: &UpstreamFailoverError{StatusCode: 503}, cancel: true, wantSelect: 1},
		{name: "no fallback preserves upstream error", err: &UpstreamFailoverError{StatusCode: 503}, exhausted: true, wantSelect: 2},
		{name: "bounded attempts", err: &UpstreamFailoverError{StatusCode: 503}, wantSelect: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var selected, forwarded, released int
			gateway := &imageStudioFailoverGatewayStub{}
			gateway.selectAccount = func(excluded map[int64]struct{}) (*AccountSelectionResult, error) {
				selected++
				if tc.exhausted && selected > 1 {
					return nil, ErrNoAvailableAccounts
				}
				require.LessOrEqual(t, selected, 3)
				require.Len(t, excluded, selected-1)
				return &AccountSelectionResult{
					Account: &Account{ID: int64(selected)}, Acquired: true,
					ReleaseFunc: func() { released++ },
				}, nil
			}
			gateway.forward = func(c *gin.Context, account *Account) (*OpenAIForwardResult, error) {
				forwarded++
				if tc.cancel {
					cancel()
				}
				if tc.write {
					c.String(http.StatusOK, "partial body")
				}
				if tc.partial {
					return &OpenAIForwardResult{ImageCount: 1}, tc.err
				}
				return nil, tc.err
			}
			result, err := newImageStudioFailoverTestExecutor(gateway).Generate(ctx, ImageStudioGenerateInput{UserID: 7, Model: "gpt-image-2"})
			require.Error(t, err)
			require.Nil(t, result)
			if !tc.cancel {
				require.ErrorIs(t, err, tc.err)
			}
			require.Equal(t, tc.wantSelect, selected)
			require.Equal(t, forwarded, released)
			require.Zero(t, gateway.recordCalls)
		})
	}
}

func TestImageStudioGatewayDoesNotRetryInterruptedBufferedImage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, readErr := range []error{io.ErrUnexpectedEOF, errors.New("stream error: stream ID 11; INTERNAL_ERROR"), ErrOpenAIUpstreamStreamTruncated} {
		t.Run(readErr.Error(), func(t *testing.T) {
			var forwarded int
			gateway := &imageStudioFailoverGatewayStub{}
			gateway.selectAccount = func(excluded map[int64]struct{}) (*AccountSelectionResult, error) {
				return &AccountSelectionResult{Account: &Account{ID: 22, Platform: PlatformOpenAI, Type: AccountTypeOAuth}}, nil
			}
			gateway.forward = func(c *gin.Context, account *Account) (*OpenAIForwardResult, error) {
				forwarded++
				response := &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body: io.NopCloser(io.MultiReader(
						strings.NewReader("data: {\"type\":\"response.image_generation_call.completed\",\"result\":\"aW1hZ2U=\"}\n\n"),
						&openAIImagesReadErrorBody{err: readErr},
					)),
				}
				service := &OpenAIGatewayService{}
				_, count, _, err := service.handleOpenAIImagesOAuthNonStreamingResponse(response, c, "b64_json", "gpt-image-2")
				require.Zero(t, count, "the reader discarded buffered image output on transport failure")
				require.Error(t, err)
				return nil, service.handleOpenAIImagesOAuthResponseError(context.Background(), c, account, "gpt-image-2", "", response, c.Writer.Size(), err)
			}
			_, err := newImageStudioFailoverTestExecutor(gateway).Generate(context.Background(), ImageStudioGenerateInput{UserID: 7, Model: "gpt-image-2"})
			require.Error(t, err)
			require.Equal(t, 1, forwarded, "a successful HTTP response with interrupted image output must not regenerate")
			require.Zero(t, gateway.recordCalls)
		})
	}
}

func TestImageStudioExecutionErrorClassifiesProviderFailures(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		reason string
	}{
		{name: "unavailable", err: &UpstreamFailoverError{StatusCode: 503, ResponseBody: []byte(`{"error":{"message":"private upstream details"}}`)}, reason: "IMAGE_PROVIDER_UNAVAILABLE"},
		{name: "limited", err: &UpstreamFailoverError{StatusCode: 429}, reason: "IMAGE_PROVIDER_RATE_LIMITED"},
		{name: "oversized failover", err: &UpstreamFailoverError{StatusCode: 413, ClientStatusCode: 413}, reason: "IMAGE_PROVIDER_REQUEST_TOO_LARGE"},
		{name: "oversized image request", err: &OpenAIImagesUpstreamError{StatusCode: 413}, reason: "IMAGE_PROVIDER_REQUEST_TOO_LARGE"},
		{name: "no account", err: ErrNoAvailableAccounts, reason: "IMAGE_STUDIO_NO_AVAILABLE_ACCOUNTS"},
		{name: "rejected", err: &OpenAIImagesUpstreamError{StatusCode: 400, Code: "content_policy_violation", Message: "private upstream details"}, reason: "IMAGE_PROVIDER_REJECTED"},
		{name: "existing error", err: infraerrors.BadRequest("EXISTING_REASON", "existing message"), reason: "EXISTING_REASON"},
		{name: "unknown internal error remains private", err: errors.New("private upstream details"), reason: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := imageStudioExecutionError("generate image", fmt.Errorf("forward image request: %w", tc.err))
			require.ErrorIs(t, err, tc.err)
			require.Equal(t, tc.reason, infraerrors.Reason(err))
			if tc.reason == "IMAGE_PROVIDER_REQUEST_TOO_LARGE" {
				require.Equal(t, http.StatusRequestEntityTooLarge, infraerrors.Code(err))
			}
			require.NotContains(t, infraerrors.Message(err), "private upstream details")
			if tc.reason != "" {
				require.NotEqual(t, "internal error", infraerrors.Message(err))
			}
		})
	}
}

func TestImageStudioTaskPersistsProviderFailureWithoutExtraGeneration(t *testing.T) {
	ctx := context.Background()
	settings := defaultImageStudioSettings()
	settings.Enabled = true
	settings.DefaultModel = "gpt-image-2"
	settings.AllowedModels = []string{"gpt-image-2"}
	tasks := &imageStudioTaskRepoStub{}
	task := &ImageStudioTask{
		UserID: 7, Mode: ImageStudioModeGeneration, Status: ImageStudioTaskStatusQueued,
		Model: "gpt-image-2", Prompt: "a blue square", AspectRatio: "1:1", Quality: ImageBillingSize1K,
	}
	require.NoError(t, tasks.Create(ctx, task))
	executor := &imageStudioExecutorStub{err: &UpstreamFailoverError{StatusCode: http.StatusServiceUnavailable}}
	svc := NewImageStudioService(&imageStudioRepoStub{}, &imageStudioConfigReaderStub{cfg: settings})
	svc.SetTaskRepository(tasks)
	svc.SetExecutor(executor)

	svc.processTask(ctx, task.ID)

	stored, err := tasks.GetByID(ctx, 7, task.ID)
	require.NoError(t, err)
	require.Equal(t, ImageStudioTaskStatusFailed, stored.Status)
	require.Equal(t, "IMAGE_PROVIDER_UNAVAILABLE", stored.ErrorReason)
	require.Contains(t, stored.ErrorMessage, "select a key from another image group")
	require.Len(t, executor.generateInputs, 1, "task recovery must not restart exhausted failover attempts")
}
