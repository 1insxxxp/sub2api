package service

import (
	"context"
	"time"
)

// UserErrorRequest 是面向终端用户的"错误请求"精简脱敏视图（白名单）。
// 严禁包含 account / api_key_prefix / upstream_endpoint / user_email 等
// 敏感或内部字段。注：message（网关标准化错误描述）与 key_name
// （用户自有 API Key 名称，KeysView 中本就可见）经产品决策对该用户开放；
// client_ip / user_agent / group_name / request_type / stream 均为该用户
// 自己请求的属性，经产品决策（2026-07-03）开放，
// 与用量明细已向用户展示自身 ip_address/user_agent/分组/类型 的口径对齐；
// 上游诊断正文仅供管理员排查。
type UserErrorRequest struct {
	ID              int64     `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	Model           string    `json:"model"`
	InboundEndpoint string    `json:"inbound_endpoint"`
	StatusCode      int       `json:"status_code"`
	ClientCanceled  bool      `json:"client_canceled"`
	Category        string    `json:"category"`
	RequestID       string    `json:"request_id,omitempty"`
	Message         string    `json:"message"`
	KeyName         string    `json:"key_name"`
	KeyDeleted      bool      `json:"key_deleted"`
	ClientIP        string    `json:"client_ip,omitempty"`
	GroupName       string    `json:"group_name,omitempty"`
	RequestType     *int16    `json:"request_type,omitempty"`
	Stream          bool      `json:"stream"`
	UserAgent       string    `json:"user_agent,omitempty"`
}

// UserErrorRequestList 是用户错误请求分页结果。
type UserErrorRequestList struct {
	Items    []*UserErrorRequest `json:"items"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// MapUserErrorCategory 把后端 error_phase + error_type 映射为用户侧粗分类码。
// 返回的是稳定的分类 code（前端做 i18n），不是展示文案。
func MapUserErrorCategory(phase, errType string) string {
	switch phase {
	case "auth":
		return "auth"
	case "routing":
		return "service_unavailable"
	case "account_auth", "upstream", "network":
		return "upstream"
	case "internal":
		return "internal"
	case "request":
		switch errType {
		case "rate_limit_error":
			return "rate_limit"
		case "billing_error", "subscription_error":
			return "quota"
		case "invalid_request_error":
			return "invalid_request"
		case "cyber_policy":
			return "cyber"
		case "client_cancel":
			return "client_cancel"
		}
	}
	return "other"
}

// CategoryToFilter 把用户侧分类码反向映射为后端过滤条件（plain ANY）。
// 未知分类返回两个空切片（即不施加分类过滤）。
// 注意："other" 与未知分类都走 default 返回空切片——"other" 无对应的 phase/type 组合，无法精确反查，因此等价于不过滤。
func CategoryToFilter(category string) (phases []string, errorTypes []string) {
	switch category {
	case "auth":
		return []string{"auth"}, nil
	case "service_unavailable":
		return []string{"routing"}, nil
	case "upstream":
		return []string{"account_auth", "upstream", "network"}, nil
	case "internal":
		return []string{"internal"}, nil
	case "rate_limit":
		return nil, []string{"rate_limit_error"}
	case "quota":
		return nil, []string{"billing_error", "subscription_error"}
	case "invalid_request":
		return nil, []string{"invalid_request_error"}
	case "cyber":
		return []string{"request"}, []string{"cyber_policy"}
	default:
		return nil, nil
	}
}

// ToUserErrorRequest 把内部 OpsErrorLog 裁剪为用户安全视图。
func ToUserErrorRequest(e *OpsErrorLog) *UserErrorRequest {
	return ToUserErrorRequestContext(context.Background(), e)
}

func ToUserErrorRequestContext(ctx context.Context, e *OpsErrorLog) *UserErrorRequest {
	if e == nil {
		return nil
	}
	model := e.RequestedModel
	if model == "" {
		model = e.Model
	}
	clientIP := ""
	if e.ClientIP != nil {
		clientIP = *e.ClientIP
	}
	category := MapUserErrorCategory(e.Phase, e.Type)
	public := userErrorPublicMessage(ctx, e, e.StatusCode, e.Type+" "+e.Message)
	statusCode := public.Status
	message := public.Message
	if e.ClientCanceled {
		category = "client_cancel"
		statusCode = e.DownstreamStatusCode
		message = "Request canceled after the connection closed; upstream usage may be unreported"
	}
	return &UserErrorRequest{
		ID:              e.ID,
		CreatedAt:       e.CreatedAt,
		Model:           model,
		InboundEndpoint: e.InboundEndpoint,
		StatusCode:      statusCode,
		ClientCanceled:  e.ClientCanceled,
		Category:        category,
		RequestID:       e.RequestID,
		Message:         message,
		KeyName:         e.APIKeyName,
		KeyDeleted:      e.APIKeyDeleted,
		ClientIP:        clientIP,
		GroupName:       e.GroupName,
		RequestType:     e.RequestType,
		Stream:          e.Stream,
		UserAgent:       e.UserAgent,
	}
}

// UserErrorRequestDetail uses the same public fields as the request list.
type UserErrorRequestDetail struct {
	UserErrorRequest
}

func ToUserErrorRequestDetail(e *OpsErrorLogDetail) *UserErrorRequestDetail {
	return ToUserErrorRequestDetailContext(context.Background(), e)
}

func ToUserErrorRequestDetailContext(ctx context.Context, e *OpsErrorLogDetail) *UserErrorRequestDetail {
	if e == nil {
		return nil
	}
	base := ToUserErrorRequestContext(ctx, &e.OpsErrorLog)
	if !e.ClientCanceled && e.UpstreamErrorMessage != "" {
		status := e.StatusCode
		if e.UpstreamStatusCode != nil {
			status = *e.UpstreamStatusCode
		}
		public := userErrorPublicMessage(ctx, &e.OpsErrorLog, status, e.UpstreamErrorMessage+" "+e.Message)
		base.Message = public.Message
		base.StatusCode = public.Status
	}
	return &UserErrorRequestDetail{UserErrorRequest: *base}
}

func userErrorPublicMessage(ctx context.Context, e *OpsErrorLog, status int, diagnostic string) PublicGatewayError {
	if (e.Phase == "auth" || e.Phase == "request") && e.Owner != "provider" && e.Source != "upstream_http" {
		if local, ok := PublicLocalGatewayError(e.StatusCode, e.Type, e.Message); ok {
			return local
		}
	}
	return ClassifyPublicGatewayErrorContext(ctx, status, diagnostic)
}
