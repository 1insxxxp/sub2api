package routes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

type categoryDefaultGroupModelResolver interface {
	ResolveCategoryDefaultGroupModel(context.Context, *service.APIKey, string) (*service.CategoryDefaultGroupModelResolution, error)
}

// This middleware runs after authentication but before the source allowlist,
// composite platform selection and any handler freezes pricing identity.
func categoryDefaultGroupTargetMiddleware(keys categoryDefaultGroupModelResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		key, ok := middleware.GetAPIKeyFromContext(c)
		if !ok || key == nil || key.Group == nil || !key.Group.IsCategoryDefault {
			c.Next()
			return
		}
		if c.Request == nil || c.Request.URL == nil || !categoryDefaultEndpointSupported(c.Request.Method, c.Request.URL.Path) {
			writeCategoryDefaultError(c, http.StatusNotImplemented, "CATEGORY_DEFAULT_ENDPOINT_UNSUPPORTED", "此入口请使用具体分组调用")
			return
		}
		if c.Request.Method == http.MethodGet {
			c.Next()
			return
		}
		if keys == nil {
			writeCategoryDefaultError(c, http.StatusServiceUnavailable, "CATEGORY_DEFAULT_UNAVAILABLE", "默认分组暂不可用")
			return
		}
		body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			status := http.StatusBadRequest
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				status = http.StatusRequestEntityTooLarge
			}
			writeCategoryDefaultError(c, status, "INVALID_REQUEST_BODY", "无法读取请求内容")
			return
		}
		if categoryDefaultHasBackgroundTask(body) {
			writeCategoryDefaultError(c, http.StatusNotImplemented, "CATEGORY_DEFAULT_ENDPOINT_UNSUPPORTED", "异步任务请使用具体分组调用")
			return
		}
		native := strings.HasPrefix(c.Request.URL.Path, "/v1beta/")
		model := ""
		if native {
			model = compositeGeminiModelFromParams(c)
			for _, candidate := range requestmodel.FromBodyCandidates(c.FullPath(), c.GetHeader("Content-Type"), body) {
				if !strings.EqualFold(strings.TrimSpace(model), strings.TrimSpace(candidate)) {
					writeCategoryDefaultError(c, http.StatusBadRequest, "AMBIGUOUS_MODEL", "请求中的模型名称不一致")
					return
				}
			}
		} else {
			var valid bool
			model, valid = uniqueCategoryDefaultRequestModel(requestmodel.FromBodyCandidates(c.FullPath(), c.GetHeader("Content-Type"), body))
			if !valid {
				writeCategoryDefaultError(c, http.StatusBadRequest, "AMBIGUOUS_MODEL", "请提供唯一且明确的模型名称")
				return
			}
		}
		if strings.TrimSpace(model) == "" {
			writeCategoryDefaultError(c, http.StatusBadRequest, "MODEL_REQUIRED", "请提供模型名称")
			return
		}
		resolution, err := keys.ResolveCategoryDefaultGroupModel(c.Request.Context(), key, model)
		if err != nil {
			status := infraerrors.Code(err)
			if status < 400 || status > 599 {
				status = http.StatusServiceUnavailable
			}
			writeCategoryDefaultError(c, status, infraerrors.Reason(err), categoryDefaultRuntimeErrorMessage(err))
			return
		}
		if resolution == nil || resolution.APIKey == nil || resolution.APIKey.Group == nil || resolution.APIKey.User == nil || resolution.APIKey.GroupID == nil || resolution.EntryGroupID != key.Group.ID || *resolution.APIKey.GroupID != resolution.APIKey.Group.ID || resolution.APIKey.Group.IsCategoryDefault || strings.TrimSpace(resolution.SourceModel) == "" {
			writeCategoryDefaultError(c, http.StatusServiceUnavailable, "CATEGORY_DEFAULT_SOURCE_UNAVAILABLE", "默认分组来源不可用")
			return
		}
		if native {
			// This capability comes from the current verified source declaration,
			// including account aliases and the effective composite route target.
			if !resolution.GeminiNative {
				writeCategoryDefaultError(c, http.StatusForbidden, "CATEGORY_DEFAULT_PROTOCOL_UNSUPPORTED", "此模型不支持 Gemini 原生接口")
				return
			}
			for i := range c.Params {
				switch c.Params[i].Key {
				case "model":
					c.Params[i].Value = resolution.SourceModel
				case "modelAction":
					value := strings.TrimPrefix(c.Params[i].Value, "/")
					action := ""
					if colon := strings.LastIndex(value, ":"); colon >= 0 {
						action = value[colon:]
					}
					c.Params[i].Value = "/" + resolution.SourceModel + action
				}
			}
		} else {
			body, err = canonicalCategoryDefaultModelBody(body, resolution.SourceModel)
			if err != nil {
				writeCategoryDefaultError(c, http.StatusBadRequest, "INVALID_REQUEST_BODY", "请求必须是有效的 JSON 对象")
				return
			}
		}
		resetRequestBody(c, body)
		c.Request = c.Request.WithContext(service.WithGroupAccountExclusions(c.Request.Context(), resolution.APIKey.User.ID, resolution.APIKey.User.GroupAccountExclusions))
		if !middleware.SetAPIKeyAndGroupContext(c, resolution.APIKey) {
			writeCategoryDefaultError(c, http.StatusServiceUnavailable, "CATEGORY_DEFAULT_SOURCE_UNAVAILABLE", "默认分组来源不可用")
			return
		}
		middleware.SetOpsFallbackAPIKey(c, resolution.APIKey)
		if native && resolution.APIKey.Group.Platform == service.PlatformAntigravity {
			// Match a concrete Antigravity source's native route without exposing
			// the forced-platform public alias on the default entry itself.
			c.Set(string(middleware.ContextKeyForcePlatform), service.PlatformAntigravity)
			c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.ForcePlatform, service.PlatformAntigravity))
		}
		c.Next()
	}
}

func categoryDefaultHasBackgroundTask(body []byte) bool {
	background := false
	gjson.ParseBytes(body).ForEach(func(key, value gjson.Result) bool {
		if strings.EqualFold(key.String(), "background") && value.Type != gjson.False && value.Type != gjson.Null {
			background = true
			return false
		}
		return true
	})
	return background
}

func canonicalCategoryDefaultModelBody(body []byte, model string) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, errors.New("request must be an object")
	}
	for key := range fields {
		if strings.EqualFold(key, "model") {
			delete(fields, key)
		}
	}
	value, _ := json.Marshal(model)
	fields["model"] = value
	return json.Marshal(fields)
}

func writeCategoryDefaultError(c *gin.Context, status int, code, message string) {
	service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalModelConfiguration)
	path := ""
	if c.Request != nil && c.Request.URL != nil {
		path = c.Request.URL.Path
	}
	if strings.HasPrefix(path, "/v1beta/") || strings.HasPrefix(path, "/antigravity/v1beta/") {
		middleware.GoogleErrorWriter(c, status, message)
		c.Abort()
		return
	}
	writeSystemCustomRootError(c, status, "invalid_request_error", code, message)
}

func categoryDefaultRuntimeErrorMessage(err error) string {
	switch {
	case errors.Is(err, service.ErrCategoryDefaultModelConflict):
		return "模型调用名重名冲突，请联系管理员调整关联来源"
	case errors.Is(err, service.ErrCategoryDefaultPendingSync):
		return "此模型的来源声明已变更，等待管理员确认同步"
	case errors.Is(err, service.ErrCategoryDefaultSourceUnavailable):
		return "来源分组暂不可用，请稍后重试或联系管理员"
	case errors.Is(err, service.ErrCategoryDefaultNotFound):
		return "默认分组已关闭或不存在，请使用具体分组调用"
	default:
		return "此模型尚未发布或当前账号无权使用，请联系管理员检查默认分组配置"
	}
}

func rejectCategoryDefaultCompositeAmbiguity(c *gin.Context, key *service.APIKey, decision service.CompositeRouteDecision) bool {
	if key == nil || key.CategoryDefaultGroupID == nil || decision.Matched ||
		(decision.Reason != service.CompositeRouteReasonAmbiguous && !decision.OAuthAmbiguous()) {
		return false
	}
	writeCategoryDefaultError(c, http.StatusBadRequest, "CATEGORY_DEFAULT_AMBIGUOUS_ROUTE", "来源分组存在模型路由歧义，请联系管理员配置明确路由")
	return true
}
