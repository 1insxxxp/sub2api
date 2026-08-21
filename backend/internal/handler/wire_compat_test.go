//go:build unit

package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestProvideAdminHandlersPreservesLegacyConstructor(t *testing.T) {
	dashboardHandler := &admin.DashboardHandler{}
	userHandler := &admin.UserHandler{}
	groupHandler := &admin.GroupHandler{}
	accountHandler := &admin.AccountHandler{}
	announcementHandler := &admin.AnnouncementHandler{}
	dataManagementHandler := &admin.DataManagementHandler{}
	backupHandler := &admin.BackupHandler{}
	oauthHandler := &admin.OAuthHandler{}
	openaiOAuthHandler := &admin.OpenAIOAuthHandler{}
	geminiOAuthHandler := &admin.GeminiOAuthHandler{}
	antigravityOAuthHandler := &admin.AntigravityOAuthHandler{}
	grokOAuthHandler := &admin.GrokOAuthHandler{}
	cnProviderHandler := &admin.CNProviderHandler{}
	proxyHandler := &admin.ProxyHandler{}
	redeemHandler := &admin.RedeemHandler{}
	promoHandler := &admin.PromoHandler{}
	settingHandler := &admin.SettingHandler{}
	opsHandler := &admin.OpsHandler{}
	systemHandler := &admin.SystemHandler{}
	subscriptionHandler := &admin.SubscriptionHandler{}
	usageHandler := &admin.UsageHandler{}
	userAttributeHandler := &admin.UserAttributeHandler{}
	errorPassthroughHandler := &admin.ErrorPassthroughHandler{}
	tlsFingerprintProfileHandler := &admin.TLSFingerprintProfileHandler{}
	apiKeyHandler := &admin.AdminAPIKeyHandler{}
	scheduledTestHandler := &admin.ScheduledTestHandler{}
	channelHandler := &admin.ChannelHandler{}
	channelMonitorHandler := &admin.ChannelMonitorHandler{}
	channelMonitorTemplateHandler := &admin.ChannelMonitorRequestTemplateHandler{}
	contentModerationHandler := &admin.ContentModerationHandler{}
	promptAuditHandler := &securityaudit.PromptAdminHandler{}
	paymentHandler := &admin.PaymentHandler{}
	affiliateHandler := &admin.AffiliateHandler{}
	complianceHandler := &admin.ComplianceHandler{}
	checkinHandler := &admin.CheckinHandler{}
	auditLogHandler := &admin.AuditLogHandler{}
	upstreamBillingProbe := &service.UpstreamBillingProbeService{}
	ollamaCloudUsage := &service.OllamaCloudUsageService{}

	got := ProvideAdminHandlers(
		dashboardHandler,
		userHandler,
		groupHandler,
		accountHandler,
		announcementHandler,
		dataManagementHandler,
		backupHandler,
		oauthHandler,
		openaiOAuthHandler,
		geminiOAuthHandler,
		antigravityOAuthHandler,
		grokOAuthHandler,
		cnProviderHandler,
		proxyHandler,
		redeemHandler,
		promoHandler,
		settingHandler,
		opsHandler,
		systemHandler,
		subscriptionHandler,
		usageHandler,
		userAttributeHandler,
		errorPassthroughHandler,
		tlsFingerprintProfileHandler,
		apiKeyHandler,
		scheduledTestHandler,
		channelHandler,
		channelMonitorHandler,
		channelMonitorTemplateHandler,
		contentModerationHandler,
		promptAuditHandler,
		paymentHandler,
		affiliateHandler,
		complianceHandler,
		checkinHandler,
		auditLogHandler,
		upstreamBillingProbe,
		ollamaCloudUsage,
	)

	require.Same(t, dashboardHandler, got.Dashboard)
	require.Same(t, userHandler, got.User)
	require.Same(t, groupHandler, got.Group)
	require.Nil(t, got.SystemCustomGroup)
	require.Same(t, accountHandler, got.Account)
	require.Same(t, announcementHandler, got.Announcement)
	require.Same(t, dataManagementHandler, got.DataManagement)
	require.Same(t, backupHandler, got.Backup)
	require.Same(t, oauthHandler, got.OAuth)
	require.Same(t, openaiOAuthHandler, got.OpenAIOAuth)
	require.Same(t, geminiOAuthHandler, got.GeminiOAuth)
	require.Same(t, antigravityOAuthHandler, got.AntigravityOAuth)
	require.Same(t, grokOAuthHandler, got.GrokOAuth)
	require.Same(t, cnProviderHandler, got.CNProvider)
	require.Same(t, proxyHandler, got.Proxy)
	require.Same(t, redeemHandler, got.Redeem)
	require.Same(t, promoHandler, got.Promo)
	require.Same(t, settingHandler, got.Setting)
	require.Same(t, opsHandler, got.Ops)
	require.Same(t, systemHandler, got.System)
	require.Same(t, subscriptionHandler, got.Subscription)
	require.Same(t, usageHandler, got.Usage)
	require.Same(t, userAttributeHandler, got.UserAttribute)
	require.Same(t, errorPassthroughHandler, got.ErrorPassthrough)
	require.Same(t, tlsFingerprintProfileHandler, got.TLSFingerprintProfile)
	require.Same(t, apiKeyHandler, got.APIKey)
	require.Same(t, scheduledTestHandler, got.ScheduledTest)
	require.Same(t, channelHandler, got.Channel)
	require.Same(t, channelMonitorHandler, got.ChannelMonitor)
	require.Same(t, channelMonitorTemplateHandler, got.ChannelMonitorTemplate)
	require.Same(t, contentModerationHandler, got.ContentModeration)
	require.Same(t, promptAuditHandler, got.PromptAudit)
	require.Same(t, paymentHandler, got.Payment)
	require.Same(t, affiliateHandler, got.Affiliate)
	require.Same(t, complianceHandler, got.Compliance)
	require.Same(t, checkinHandler, got.Checkin)
	require.Same(t, auditLogHandler, got.AuditLog)

	systemCustomGroupHandler := &admin.SystemCustomGroupHandler{}
	withSystemCustom := ProvideAdminHandlersWithSystemCustomGroup(
		dashboardHandler,
		userHandler,
		groupHandler,
		accountHandler,
		announcementHandler,
		dataManagementHandler,
		backupHandler,
		oauthHandler,
		openaiOAuthHandler,
		geminiOAuthHandler,
		antigravityOAuthHandler,
		grokOAuthHandler,
		cnProviderHandler,
		proxyHandler,
		redeemHandler,
		promoHandler,
		settingHandler,
		opsHandler,
		systemHandler,
		subscriptionHandler,
		usageHandler,
		userAttributeHandler,
		errorPassthroughHandler,
		tlsFingerprintProfileHandler,
		apiKeyHandler,
		scheduledTestHandler,
		channelHandler,
		channelMonitorHandler,
		channelMonitorTemplateHandler,
		contentModerationHandler,
		promptAuditHandler,
		paymentHandler,
		affiliateHandler,
		complianceHandler,
		checkinHandler,
		auditLogHandler,
		upstreamBillingProbe,
		ollamaCloudUsage,
		systemCustomGroupHandler,
	)
	require.Same(t, systemCustomGroupHandler, withSystemCustom.SystemCustomGroup)
	require.Same(t, dashboardHandler, withSystemCustom.Dashboard)
	require.Same(t, accountHandler, withSystemCustom.Account)
}
