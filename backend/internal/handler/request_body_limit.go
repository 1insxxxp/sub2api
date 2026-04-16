package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

func extractMaxBytesError(err error) (*http.MaxBytesError, bool) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return maxErr, true
	}
	return nil, false
}

func formatBodyLimit(limit int64) string {
	const mb = 1024 * 1024
	if limit >= mb {
		return fmt.Sprintf("%dMB", limit/mb)
	}
	return fmt.Sprintf("%dB", limit)
}

func buildBodyTooLargeMessage(limit int64) string {
	return fmt.Sprintf("Request body too large, limit is %s", formatBodyLimit(limit))
}

func classifyRequestBodyReadError(err error) (int, string, string) {
	if err == nil {
		return http.StatusBadRequest, "invalid_request_error", "Failed to read request body"
	}
	if errors.Is(err, context.Canceled) {
		return http.StatusBadRequest, "invalid_request_error", "Client canceled request while sending request body"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusRequestTimeout, "invalid_request_error", "Request body read timed out"
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return http.StatusRequestTimeout, "invalid_request_error", "Request body read timed out"
	}

	errMsg := strings.ToLower(strings.TrimSpace(err.Error()))
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) || strings.Contains(errMsg, "unexpected eof") {
		return http.StatusBadRequest, "invalid_request_error", "Request body stream interrupted"
	}

	return http.StatusBadRequest, "invalid_request_error", "Failed to read request body"
}
