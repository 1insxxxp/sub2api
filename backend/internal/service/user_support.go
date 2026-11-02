package service

import (
	"context"
	"errors"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const UserSupportTrialAmount = 15

var (
	ErrUserSupportForbidden      = errors.New("user support access denied")
	ErrUserSupportUserNotFound   = errors.New("user support user not found")
	ErrUserSupportInvalidAccount = errors.New("invalid user support account")
	ErrUserSupportUnavailable    = errors.New("user support temporarily unavailable")
)

// These DTOs are deliberately independent of both admin and ordinary-user
// history DTOs. No future upstream/debug field can implicitly become public.
type UserSupportUser struct {
	ID      int64   `json:"id"`
	Email   string  `json:"email"`
	Balance float64 `json:"balance"`
}
type UserSupportSuggestion struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}
type UserSupportTrial struct {
	Status    string    `json:"status"`
	Amount    int       `json:"amount"`
	GrantedAt time.Time `json:"granted_at"`
	Legacy    bool      `json:"legacy"`
}
type UserSupportGrantResult struct {
	User  UserSupportUser  `json:"user"`
	Trial UserSupportTrial `json:"trial"`
}
type UserSupportUsageRecord struct {
	ID              int64     `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	Model           string    `json:"model"`
	GroupName       string    `json:"group_name"`
	InputTokens     *int64    `json:"input_tokens"`
	OutputTokens    *int64    `json:"output_tokens"`
	CacheReadTokens *int64    `json:"cache_read_tokens"`
	ChargedAmount   float64   `json:"charged_amount"`
	DurationMs      *int64    `json:"duration_ms"`
	FirstTokenMs    *int64    `json:"first_token_ms"`
	OutputTPS       *float64  `json:"output_tps"`
	Status          string    `json:"status"`
	Stream          bool      `json:"stream"`
}
type UserSupportErrorRecord struct {
	ID              int64     `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	Model           string    `json:"model"`
	GroupName       string    `json:"group_name"`
	StatusCode      *int64    `json:"status_code"`
	ErrorKind       string    `json:"error_kind"`
	Category        string    `json:"category"`
	OriginalMessage string    `json:"original_message"`
}
type UserSupportUsagePanel struct {
	Status string                   `json:"status"`
	Items  []UserSupportUsageRecord `json:"items"`
}
type UserSupportErrorPanel struct {
	Status string                   `json:"status"`
	Items  []UserSupportErrorRecord `json:"items"`
}
type UserSupportResult struct {
	User   UserSupportUser       `json:"user"`
	Trial  UserSupportTrial      `json:"trial"`
	Usage  UserSupportUsagePanel `json:"usage"`
	Errors UserSupportErrorPanel `json:"errors"`
}

type UserSupportRepository interface {
	CanAccess(context.Context, int64) (bool, error)
	GetAccess(context.Context, int64, int64) (bool, error)
	SetAccess(context.Context, int64, int64, bool) error
	Search(context.Context, int64, string) ([]UserSupportSuggestion, error)
	Lookup(context.Context, int64, string) (*UserSupportGrantResult, error)
	LookupAndGrant(context.Context, int64, string) (*UserSupportGrantResult, error)
	ListUsage(context.Context, int64) ([]UserSupportUsageRecord, error)
	ListErrors(context.Context, int64) ([]UserSupportErrorRecord, error)
}

type userSupportBalanceInvalidator interface {
	InvalidateUserBalance(context.Context, int64) error
}
type UserSupportService struct {
	repo         UserSupportRepository
	billingCache userSupportBalanceInvalidator
	authCache    APIKeyAuthCacheInvalidator
}

func NewUserSupportService(repo UserSupportRepository, billingCache *BillingCacheService, apiKeys *APIKeyService) *UserSupportService {
	s := &UserSupportService{repo: repo}
	if billingCache != nil {
		s.billingCache = billingCache
	}
	if apiKeys != nil {
		s.authCache = apiKeys
	}
	return s
}

func (s *UserSupportService) CanAccess(ctx context.Context, actorID int64) (bool, error) {
	if actorID <= 0 {
		return false, ErrUserSupportForbidden
	}
	ok, err := s.repo.CanAccess(ctx, actorID)
	return ok, safeUserSupportError(err)
}
func (s *UserSupportService) GetAccess(ctx context.Context, actorID, targetID int64) (bool, error) {
	if actorID <= 0 || targetID <= 0 {
		return false, ErrUserSupportForbidden
	}
	ok, err := s.repo.GetAccess(ctx, actorID, targetID)
	return ok, safeUserSupportError(err)
}
func (s *UserSupportService) SetAccess(ctx context.Context, actorID, targetID int64, enabled bool) error {
	if actorID <= 0 || targetID <= 0 {
		return ErrUserSupportForbidden
	}
	return safeUserSupportError(s.repo.SetAccess(ctx, actorID, targetID, enabled))
}
func (s *UserSupportService) requireSupportAccess(ctx context.Context, actorID int64) error {
	ok, err := s.CanAccess(ctx, actorID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrUserSupportForbidden
	}
	return nil
}

// Search is a bounded identity-only read. It never checks out or consumes a
// trial grant, and never changes either balance or authentication caches.
func (s *UserSupportService) Search(ctx context.Context, actorID int64, query string) ([]UserSupportSuggestion, error) {
	if err := s.requireSupportAccess(ctx, actorID); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" || len(query) > 254 || strings.IndexFunc(query, unicode.IsControl) >= 0 {
		return nil, ErrUserSupportInvalidAccount
	}
	rows, err := s.repo.Search(ctx, actorID, query)
	if err != nil {
		return nil, safeUserSupportError(err)
	}
	items := make([]UserSupportSuggestion, 0, min(len(rows), 8))
	for _, row := range rows {
		if row.ID <= 0 || !strings.Contains(row.Email, "@") || !validUserSupportAccount(row.Email) {
			continue
		}
		items = append(items, row)
		if len(items) == 8 {
			break
		}
	}
	return items, nil
}

// Grant is the sole financial entry point. The displayed lookup snapshot is
// advisory: the original repository transaction rechecks eligibility and access.
func (s *UserSupportService) Grant(ctx context.Context, actorID, userID int64) (*UserSupportGrantResult, error) {
	if err := s.requireSupportAccess(ctx, actorID); err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, ErrUserSupportInvalidAccount
	}
	grant, err := s.repo.LookupAndGrant(ctx, actorID, strconv.FormatInt(userID, 10))
	if err != nil {
		return nil, safeUserSupportError(err)
	}
	if grant == nil {
		return nil, ErrUserSupportUnavailable
	}
	// Also invalidate on replay: a previous process may have committed the grant
	// and exited before invalidation. Never report a committed credit as failed.
	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if s.authCache != nil {
		s.authCache.InvalidateAuthCacheByUserID(cacheCtx, grant.User.ID)
	}
	if s.billingCache != nil {
		_ = s.billingCache.InvalidateUserBalance(cacheCtx, grant.User.ID)
	}
	return grant, nil
}

func (s *UserSupportService) Lookup(ctx context.Context, actorID int64, account string) (*UserSupportResult, error) {
	if err := s.requireSupportAccess(ctx, actorID); err != nil {
		return nil, err
	}
	account = strings.TrimSpace(account)
	if !validUserSupportAccount(account) {
		return nil, ErrUserSupportInvalidAccount
	}
	grant, err := s.repo.Lookup(ctx, actorID, account)
	if err != nil {
		return nil, safeUserSupportError(err)
	}
	if grant == nil {
		return nil, ErrUserSupportUnavailable
	}

	result := &UserSupportResult{User: grant.User, Trial: grant.Trial,
		Usage:  UserSupportUsagePanel{Status: "unavailable", Items: []UserSupportUsageRecord{}},
		Errors: UserSupportErrorPanel{Status: "unavailable", Items: []UserSupportErrorRecord{}}}
	if rows, readErr := s.repo.ListUsage(ctx, grant.User.ID); readErr == nil {
		result.Usage.Status = "ok"
		for i := 0; i < len(rows) && i < 10; i++ {
			row := rows[i]
			row.Model = PublicUserSupportModel(row.Model)
			row.GroupName = PublicUserSupportGroupName(row.GroupName)
			switch row.Status {
			case "completed", "interrupted", "failed", "empty":
			default:
				row.Status = "unknown"
			}
			result.Usage.Items = append(result.Usage.Items, row)
		}
	}
	if rows, readErr := s.repo.ListErrors(ctx, grant.User.ID); readErr == nil {
		result.Errors.Status = "ok"
		for i := 0; i < len(rows) && i < 10; i++ {
			row := rows[i]
			row.Model = PublicUserSupportModel(row.Model)
			row.GroupName = PublicUserSupportGroupName(row.GroupName)
			row.ErrorKind = PublicUserSupportErrorKind(row.ErrorKind)
			row.OriginalMessage = PublicUserSupportOriginalMessage(row.OriginalMessage, "")
			if row.StatusCode != nil && (*row.StatusCode < 100 || *row.StatusCode > 599) {
				row.StatusCode = nil
			}
			switch row.Category {
			case "auth", "quota", "rate_limit", "invalid_request", "timeout", "service_unavailable":
			default:
				row.Category = "other"
			}
			result.Errors.Items = append(result.Errors.Items, row)
		}
	}
	return result, nil
}

func safeUserSupportError(err error) error {
	if err == nil {
		return nil
	}
	for _, known := range []error{ErrUserSupportForbidden, ErrUserSupportUserNotFound, ErrUserSupportInvalidAccount} {
		if errors.Is(err, known) {
			return known
		}
	}
	return ErrUserSupportUnavailable
}
func validUserSupportAccount(value string) bool {
	if value == "" || len(value) > 254 || strings.ContainsAny(value, "\r\n\x00") {
		return false
	}
	if !strings.Contains(value, "@") {
		id, err := strconv.ParseInt(value, 10, 64)
		return err == nil && id > 0 && strconv.FormatInt(id, 10) == value
	}
	parsed, err := mail.ParseAddress(value)
	return err == nil && parsed.Address == value
}

// Only static, reviewed public model names may be shown. Product prefixes and
// arbitrary client-controlled values cannot reflect an address or credential.
func PublicUserSupportModel(value string) string {
	value = strings.TrimSpace(value)
	if i := strings.LastIndex(value, "/"); i >= 0 {
		value = value[i+1:]
	}
	for _, model := range userSupportPublicModels {
		if strings.EqualFold(value, model) {
			return model
		}
	}
	return "unknown"
}

var userSupportPublicModels = mergeDefaultModelIDs(DefaultModelIDsForPlatform(PlatformComposite), []string{
	"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.5-flash-lite",
	"gemini-4-argon", "gemini-3.1-pro-preview", "gemini-3.5-flash", "gemini-3.6-flash", "gemini-3.7-flash", "gemini-3.8-flash",
})

func UserSupportErrorCategory(phase, kind string) string {
	if kind == "timeout" || kind == "timeout_error" || kind == "request_timeout" {
		return "timeout"
	}
	switch MapUserErrorCategory(phase, kind) {
	case "auth":
		return "auth"
	case "quota":
		return "quota"
	case "rate_limit":
		return "rate_limit"
	case "invalid_request":
		return "invalid_request"
	case "upstream", "internal", "service_unavailable":
		return "service_unavailable"
	default:
		return "other"
	}
}

// PublicUserSupportErrorKind exposes only the reviewed error-stage labels used
// by the support page. Historical phase/type strings are never reflected.
func PublicUserSupportErrorKind(value string) string {
	switch value {
	case "request", "auth", "account_auth", "routing", "upstream", "network", "internal":
		return value
	default:
		return "unknown"
	}
}
