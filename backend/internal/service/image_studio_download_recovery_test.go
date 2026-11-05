//go:build unit

package service

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type imageStudioDownloadRecoveryGateway struct {
	*imageStudioGIFGateway
	fetchErrors []error
	onFetch     func()
}

func (g *imageStudioDownloadRecoveryGateway) fetchOpenAIImageURLBase64(_ context.Context, _ *Account, url string) (string, error) {
	g.fetchURLs = append(g.fetchURLs, url)
	if g.onFetch != nil {
		g.onFetch()
	}
	if index := len(g.fetchURLs) - 1; index < len(g.fetchErrors) && g.fetchErrors[index] != nil {
		return "", g.fetchErrors[index]
	}
	return base64.StdEncoding.EncodeToString(g.data), nil
}

func imageStudioDownloadRecoveryFixture(t *testing.T) (*ImageStudioService, *imageStudioDownloadRecoveryGateway, *imageStudioRepoStub, *imageStudioStorageStub, ImageStudioGenerateInput) {
	t.Helper()
	gateway := &imageStudioDownloadRecoveryGateway{imageStudioGIFGateway: &imageStudioGIFGateway{
		data:     imageStudioAnimatedGIFFixture(t),
		url:      "https://cdn.example.com/generated.gif",
		accounts: []Account{imageStudioGIFAccount(1, "gpt-image-2.6-flare")},
	}}
	repo, storage := &imageStudioRepoStub{}, &imageStudioStorageStub{}
	svc := NewImageStudioService(repo, &imageStudioConfigReaderStub{cfg: &ImageStudioSettings{Enabled: true}})
	svc.SetGroupResolver(&imageStudioGroupResolverStub{groups: []Group{{ID: 9, Status: StatusActive, Platform: PlatformOpenAI, AllowImageGeneration: true}}})
	svc.SetModelDiscovery(&imageStudioDiscoveryAccounts{accounts: gateway.accounts}, nil)
	svc.SetExecutor(imageStudioGIFExecutor(gateway))
	svc.SetStorageFactory(func(context.Context, *ImageStudioSettings) (ImageStorage, error) { return storage, nil })
	input := ImageStudioGenerateInput{UserID: 7, Model: "绘图/loop", Prompt: "loop", OutputFormat: "gif"}
	return svc, gateway, repo, storage, input
}

func TestImageStudioGIFDownloadFailureDoesNotRegenerateOrCharge(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cause    error
		attempts int
	}{
		{"timeout", context.DeadlineExceeded, 2},
		{"truncated_download", io.ErrUnexpectedEOF, 2},
		{"blocked_host", errors.New("image url host is not allowed"), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, gateway, repo, storage, input := imageStudioDownloadRecoveryFixture(t)
			gateway.fetchErrors = []error{tc.cause, tc.cause, tc.cause, tc.cause}
			image, _, _, err := svc.generateTaskWithRecovery(context.Background(), input, 0)
			require.Error(t, err)
			require.Nil(t, image)
			require.Equal(t, []int64{1}, gateway.forwarded, "a completed generation must never be retried after download failure")
			require.Len(t, gateway.fetchURLs, tc.attempts)
			for _, url := range gateway.fetchURLs {
				require.Equal(t, gateway.url, url)
			}
			require.ErrorIs(t, err, tc.cause, "retain the download cause for diagnostics")
			require.Equal(t, "IMAGE_RESULT_DOWNLOAD_FAILED", infraerrors.Reason(err))
			require.Contains(t, infraerrors.Message(err), "generation was not retried")
			require.Zero(t, gateway.recordCalls)
			require.Empty(t, repo.createdRecords)
			require.Empty(t, storage.putData)
		})
	}
}

func TestImageStudioGIFDownloadRetryUsesSameURLAndChargesOnlyOnSuccess(t *testing.T) {
	for _, storageFails := range []bool{false, true} {
		svc, gateway, repo, storage, input := imageStudioDownloadRecoveryFixture(t)
		gateway.fetchErrors = []error{context.DeadlineExceeded}
		if storageFails {
			storage.putErr = errors.New("storage unavailable")
		}
		image, _, _, err := svc.generateTaskWithRecovery(context.Background(), input, 0)
		require.Equal(t, []int64{1}, gateway.forwarded)
		require.Equal(t, []string{gateway.url, gateway.url}, gateway.fetchURLs)
		require.Equal(t, gateway.data, storage.putData, "download retry must preserve every GIF byte")
		if storageFails {
			require.ErrorContains(t, err, "storage unavailable")
			require.Nil(t, image)
			require.Zero(t, gateway.recordCalls)
			require.Empty(t, repo.createdRecords)
		} else {
			require.NoError(t, err)
			require.Equal(t, "image/gif", image.MimeType)
			require.Equal(t, 1, gateway.recordCalls)
			require.Len(t, repo.createdRecords, 1)
		}
	}
}

func TestImageStudioGIFDownloadCancellationStopsRetry(t *testing.T) {
	svc, gateway, _, _, input := imageStudioDownloadRecoveryFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gateway.onFetch = cancel
	gateway.fetchErrors = []error{context.DeadlineExceeded}
	_, _, _, err := svc.generateTaskWithRecovery(ctx, input, 0)
	require.Error(t, err)
	require.Equal(t, []int64{1}, gateway.forwarded)
	require.Equal(t, []string{gateway.url}, gateway.fetchURLs)
	require.Zero(t, gateway.recordCalls)
}

func TestImageStudioGIFDownloadFailurePersistsSpecificTaskReason(t *testing.T) {
	svc, gateway, _, _, input := imageStudioDownloadRecoveryFixture(t)
	gateway.fetchErrors = []error{context.DeadlineExceeded, context.DeadlineExceeded}
	tasks := &imageStudioTaskRepoStub{}
	svc.SetTaskRepository(tasks)
	task := svc.taskFromGenerateInput(nil, input)
	require.NoError(t, tasks.Create(context.Background(), task))
	svc.processTask(context.Background(), task.ID)
	finished, err := tasks.GetByID(context.Background(), input.UserID, task.ID)
	require.NoError(t, err)
	require.Equal(t, ImageStudioTaskStatusFailed, finished.Status)
	require.Equal(t, "IMAGE_RESULT_DOWNLOAD_FAILED", finished.ErrorReason)
	require.Contains(t, finished.ErrorMessage, "generation was not retried")
	require.Equal(t, []int64{1}, gateway.forwarded)
	require.Zero(t, gateway.recordCalls)
}
