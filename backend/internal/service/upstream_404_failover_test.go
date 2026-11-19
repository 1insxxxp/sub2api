package service

import (
	"net/http"
	"testing"
)

func TestOpenAIHTTP404Failover(t *testing.T) {
	body := []byte("404 page not found")
	service := &OpenAIGatewayService{}
	if !service.shouldFailoverOpenAIUpstreamResponse(nil, http.StatusNotFound, string(body), body) ||
		!shouldFailoverOpenAIPassthroughResponse(nil, http.StatusNotFound, body) {
		t.Fatal("a generic upstream 404 should allow selecting the next account")
	}
}
