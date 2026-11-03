package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// OpenAIContinuationAlias contains only routing scalars, never transcript data.
// Keys include the billing group and authenticated downstream user.
type OpenAIContinuationAlias struct {
	ResponseID string `json:"response_id"`
	AccountID  int64  `json:"account_id"`
}

// Optional interface keeps existing GatewayCache implementations compatible.
type OpenAIContinuationAliasCache interface {
	SetOpenAIContinuationAlias(context.Context, int64, int64, string, OpenAIContinuationAlias, time.Duration) error
	GetOpenAIContinuationAlias(context.Context, int64, int64, string) (OpenAIContinuationAlias, bool, error)
}
type openAIContinuationAliasBinding struct {
	alias     OpenAIContinuationAlias
	expiresAt time.Time
}

func openAIContinuationAliasKey(groupID, userID int64, public string) string {
	digest := sha256.Sum256([]byte(public))
	return fmt.Sprintf("%d:%d:%s", groupID, userID, hex.EncodeToString(digest[:]))
}
func (s *defaultOpenAIWSStateStore) bindContinuationAlias(ctx context.Context, groupID, userID int64, public string, b OpenAIContinuationAlias, ttl time.Duration) error {
	if userID <= 0 || public == "" || b.ResponseID == "" || b.AccountID <= 0 {
		return errors.New("invalid continuation alias binding")
	}
	ttl = normalizeOpenAIWSTTL(ttl)
	if cache, ok := s.cache.(OpenAIContinuationAliasCache); ok {
		if err := cache.SetOpenAIContinuationAlias(ctx, groupID, userID, public, b, ttl); err != nil {
			return err
		}
	}
	key := openAIContinuationAliasKey(groupID, userID, public)
	s.continuationAliasMu.Lock()
	defer s.continuationAliasMu.Unlock()
	if s.continuationAliases == nil {
		s.continuationAliases = make(map[string]openAIContinuationAliasBinding)
	}
	ensureBindingCapacity(s.continuationAliases, key, openAIWSStateStoreMaxEntriesPerMap)
	s.continuationAliases[key] = openAIContinuationAliasBinding{alias: b, expiresAt: time.Now().Add(ttl)}
	return nil
}
func (s *defaultOpenAIWSStateStore) resolveContinuationAlias(ctx context.Context, groupID, userID int64, public string) (OpenAIContinuationAlias, bool, error) {
	if cache, ok := s.cache.(OpenAIContinuationAliasCache); ok {
		// Redis remains authoritative, including expiry or deletion on another node.
		return cache.GetOpenAIContinuationAlias(ctx, groupID, userID, public)
	}
	key := openAIContinuationAliasKey(groupID, userID, public)
	s.continuationAliasMu.Lock()
	defer s.continuationAliasMu.Unlock()
	b, ok := s.continuationAliases[key]
	if ok && time.Now().Before(b.expiresAt) {
		return b.alias, true, nil
	}
	delete(s.continuationAliases, key)
	return OpenAIContinuationAlias{}, false, nil
}
func (s *OpenAIGatewayService) bindContinuationAlias(ctx context.Context, groupID, userID int64, public string, b OpenAIContinuationAlias) error {
	store, ok := s.getOpenAIWSStateStore().(*defaultOpenAIWSStateStore)
	if !ok {
		return errors.New("continuation aliases unavailable in custom state store")
	}
	return store.bindContinuationAlias(ctx, groupID, userID, public, b, s.openAIWSResponseStickyTTL())
}

// ResolveOpenAIContinuationAlias must be called after authorizing the public
// response owner. User scoping additionally prevents cross-tenant resolution.
func (s *OpenAIGatewayService) ResolveOpenAIContinuationAlias(ctx context.Context, groupID, userID int64, public string) (string, error) {
	store, ok := s.getOpenAIWSStateStore().(*defaultOpenAIWSStateStore)
	if !ok || userID <= 0 {
		return public, nil
	}
	binding, found, err := store.resolveContinuationAlias(ctx, groupID, userID, public)
	if err != nil {
		return "", err
	}
	if !found {
		return public, nil
	}
	if strings.TrimSpace(binding.ResponseID) == "" || binding.AccountID <= 0 {
		return "", errors.New("invalid continuation alias")
	}
	return binding.ResponseID, nil
}
func openAIContinuationOwner(c *gin.Context) (userID, keyID int64) {
	if key := getAPIKeyFromContext(c); key != nil && key.UserID > 0 {
		return key.UserID, key.ID
	}
	if raw, ok := c.Get(openAIHTTPResponseOwnerContextKey); ok {
		if owner, ok := raw.(openAIHTTPResponseOwner); ok {
			return owner.userID, owner.apiKeyID
		}
	}
	return 0, 0
}
func (s *OpenAIGatewayService) bindOpenAIContinuationAlias(ctx context.Context, c *gin.Context, account *Account, result *OpenAIForwardResult) error {
	if result == nil || result.AntiTruncationRounds <= 0 || result.ContinuationPublicResponseID == "" {
		return nil
	}
	public := result.ContinuationPublicResponseID
	actual := strings.TrimSpace(result.ResponseID)
	if actual == "" {
		actual = strings.TrimSpace(result.RequestID)
	}
	if actual == "" || actual == public {
		return nil
	}
	userID, keyID := openAIContinuationOwner(c)
	if userID <= 0 || keyID <= 0 {
		return errors.New("continuation owner is unavailable")
	}
	bindCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAIWSStateStoreRedisTimeout)
	defer cancel()
	groupID := getOpenAIGroupIDFromContext(c)
	s.bindHTTPResponseAccount(bindCtx, c, account, public)
	s.bindHTTPResponseAccount(bindCtx, c, account, actual)
	if err := s.BindOpenAIHTTPResponseOwner(bindCtx, groupID, public, userID, keyID); err != nil {
		return err
	}
	return s.bindContinuationAlias(bindCtx, groupID, userID, public, OpenAIContinuationAlias{ResponseID: actual, AccountID: account.ID})
}
