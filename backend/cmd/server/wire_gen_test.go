package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestProvideServiceBuildInfo(t *testing.T) {
	in := handler.BuildInfo{
		Version:   "v-test",
		BuildType: "release",
	}
	out := provideServiceBuildInfo(in)
	require.Equal(t, in.Version, out.Version)
	require.Equal(t, in.BuildType, out.BuildType)
}

func TestProvideCleanup_WithMinimalDependencies_NoPanic(t *testing.T) {
	cfg := &config.Config{}

	oauthSvc := service.NewOAuthService(nil, nil)
	openAIOAuthSvc := service.NewOpenAIOAuthService(nil, nil)
	geminiOAuthSvc := service.NewGeminiOAuthService(nil, nil, nil, nil, cfg)
	antigravityOAuthSvc := service.NewAntigravityOAuthService(nil)

	tokenRefreshSvc := service.NewTokenRefreshService(
		nil,
		oauthSvc,
		openAIOAuthSvc,
		geminiOAuthSvc,
		antigravityOAuthSvc,
		nil,
		nil,
		cfg,
		nil,
	)
	accountExpirySvc := service.NewAccountExpiryService(nil, time.Second)
	codexVersionSyncSvc := service.NewOpenAICodexVersionSyncService(nil, nil, nil, time.Second)
	claudeCodeVersionSyncSvc := service.NewClaudeCodeVersionSyncService(nil, nil, nil, time.Second)
	proxyExpirySvc := service.NewProxyExpiryService(nil, time.Second)
	subscriptionExpirySvc := service.NewSubscriptionExpiryService(nil, time.Second)
	pricingSvc := service.NewPricingService(cfg, nil)
	emailQueueSvc := service.NewEmailQueueService(nil, 1)
	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	idempotencyCleanupSvc := service.NewIdempotencyCleanupService(nil, cfg)
	schedulerSnapshotSvc := service.NewSchedulerSnapshotService(nil, nil, nil, nil, cfg)
	accountPoolHealthSvc := service.NewAccountPoolHealthNotifyService(nil, nil, nil, time.Minute)
	quotaRepo := &quotaRefreshCleanupRepo{}
	quotaRefreshSvc := service.NewOpenAIQuotaRefreshService(quotaRepo, &service.AccountUsageService{}, 5*time.Millisecond)
	quotaRefreshSvc.Start()
	t.Cleanup(quotaRefreshSvc.Stop)
	require.Eventually(t, func() bool { return quotaRepo.calls.Load() >= 2 }, time.Second, time.Millisecond)
	opsSystemLogSinkSvc := service.NewOpsSystemLogSink(nil)

	cleanup := provideCleanup(
		nil, // entClient
		nil, // redis
		&service.OpsMetricsCollector{},
		&service.OpsAggregationService{},
		&service.OpsAlertEvaluatorService{},
		&service.OpsCleanupService{},
		&service.OpsScheduledReportService{},
		opsSystemLogSinkSvc,
		nil, // opsService
		nil, // opsIngressRejectAggregator
		nil, // apiKeyService
		nil, // authCacheInvalidationWorker
		schedulerSnapshotSvc,
		accountPoolHealthSvc,
		quotaRefreshSvc,
		tokenRefreshSvc,
		accountExpirySvc,
		nil, // cnProviderBalanceCheck
		codexVersionSyncSvc,
		claudeCodeVersionSyncSvc,
		proxyExpirySvc,
		subscriptionExpirySvc,
		&service.UsageCleanupService{},
		idempotencyCleanupSvc,
		service.NewGeminiCachedContentService(nil, nil, cfg),
		&service.BatchImageCleanupService{},
		nil, // batchImageWorker
		pricingSvc,
		emailQueueSvc,
		billingCacheSvc,
		&service.UsageRecordWorkerPool{},
		service.ProvideBalanceNotifyService(nil, nil, nil, nil),
		service.NewDigestSessionStore(),
		&service.SubscriptionService{},
		oauthSvc,
		openAIOAuthSvc,
		geminiOAuthSvc,
		antigravityOAuthSvc,
		nil, // grokOAuth
		nil, // openAIGateway
		nil, // scheduledTestRunner
		nil, // backupSvc
		nil, // paymentOrderExpiry
		nil, // channelMonitorRunner
		nil, // channelMonitorV2Aggregator
		nil, // quotaFlusher
		nil, // upstreamBillingProbe
		nil, // ollamaCloudUsage
		nil, // opencodeGoUsage
		nil, // auditLog
		nil, // openAIAutoReset
		nil, // promptAudit
		nil, // pluginManager
	)

	require.NotPanics(t, func() {
		cleanup()
	})
	calls := quotaRepo.calls.Load()
	require.Never(t, func() bool { return quotaRepo.calls.Load() != calls }, 30*time.Millisecond, time.Millisecond)
}

type quotaRefreshCleanupRepo struct {
	service.AccountRepository
	calls atomic.Int64
}

func (r *quotaRefreshCleanupRepo) ListByPlatform(context.Context, string) ([]service.Account, error) {
	r.calls.Add(1)
	return nil, nil
}

func TestGeneratedWireConnectsOpenAIQuotaRefresh(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "wire_gen.go", nil, 0)
	require.NoError(t, err)
	var providedName string
	var cleanupArgs []ast.Expr
	ast.Inspect(file, func(node ast.Node) bool {
		if assign, ok := node.(*ast.AssignStmt); ok && len(assign.Rhs) == 1 {
			if call, ok := assign.Rhs[0].(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "ProvideOpenAIQuotaRefreshService" {
					require.Len(t, assign.Lhs, 1)
					provided, ok := assign.Lhs[0].(*ast.Ident)
					require.True(t, ok)
					providedName = provided.Name
					require.Len(t, call.Args, 2)
				}
			}
		}
		if call, ok := node.(*ast.CallExpr); ok {
			if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "provideCleanup" {
				cleanupArgs = call.Args
			}
		}
		return true
	})
	require.NotEmpty(t, providedName, "production injector must construct and start the quota refresher")
	connected := false
	for _, arg := range cleanupArgs {
		if name, ok := arg.(*ast.Ident); ok && name.Name == providedName {
			connected = true
		}
	}
	require.True(t, connected, "the started instance must be owned by server cleanup")
}
