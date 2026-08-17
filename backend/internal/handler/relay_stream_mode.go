package handler

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type openAICompatibleRelayStreamMode string

const (
	openAICompatibleRelayStreamModeNormal         openAICompatibleRelayStreamMode = ""
	openAICompatibleRelayStreamModeForceStream    openAICompatibleRelayStreamMode = "force_stream"
	openAICompatibleRelayStreamModeForceNonStream openAICompatibleRelayStreamMode = "force_nonstream"
)

const openAICompatibleRelayStreamModeContextKey = "_openai_compatible_relay_stream_mode"

func markOpenAICompatibleRelayStreamMode(c *gin.Context, mode openAICompatibleRelayStreamMode) {
	if c == nil || mode == openAICompatibleRelayStreamModeNormal {
		return
	}
	c.Set(openAICompatibleRelayStreamModeContextKey, mode)
}

func OpenAICompatibleRelayForceStreamMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		markOpenAICompatibleRelayStreamMode(c, openAICompatibleRelayStreamModeForceStream)
		c.Next()
	}
}

func OpenAICompatibleRelayForceNonStreamMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		markOpenAICompatibleRelayStreamMode(c, openAICompatibleRelayStreamModeForceNonStream)
		c.Next()
	}
}

func openAICompatibleRelayStreamModeFromContext(c *gin.Context) openAICompatibleRelayStreamMode {
	if c == nil {
		return openAICompatibleRelayStreamModeNormal
	}
	value, exists := c.Get(openAICompatibleRelayStreamModeContextKey)
	if !exists {
		return openAICompatibleRelayStreamModeNormal
	}
	if mode, ok := value.(openAICompatibleRelayStreamMode); ok {
		return mode
	}
	if raw, ok := value.(string); ok {
		switch openAICompatibleRelayStreamMode(strings.TrimSpace(raw)) {
		case openAICompatibleRelayStreamModeForceStream:
			return openAICompatibleRelayStreamModeForceStream
		case openAICompatibleRelayStreamModeForceNonStream:
			return openAICompatibleRelayStreamModeForceNonStream
		}
	}
	return openAICompatibleRelayStreamModeNormal
}

func applyOpenAICompatibleRelayStreamMode(c *gin.Context, body []byte) ([]byte, bool, error) {
	mode := openAICompatibleRelayStreamModeFromContext(c)
	if mode == openAICompatibleRelayStreamModeNormal {
		return body, false, nil
	}
	if !gjson.ValidBytes(body) {
		return body, false, errInvalidRelayStreamModeJSON
	}

	forceStream := mode == openAICompatibleRelayStreamModeForceStream
	rewritten, err := sjson.SetBytes(body, "stream", forceStream)
	if err != nil {
		return body, false, err
	}
	return rewritten, string(rewritten) != string(body), nil
}

type relayStreamModeError string

func (e relayStreamModeError) Error() string { return string(e) }

const errInvalidRelayStreamModeJSON relayStreamModeError = "invalid JSON body"
