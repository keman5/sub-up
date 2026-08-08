.PHONY: build build-backend build-frontend build-datamanagementd test test-backend test-frontend test-frontend-critical test-datamanagementd secret-scan fork-check fork-inventory fork-snapshot fork-verify fork-restore-dry-run deploy-gzip-dry-run

FRONTEND_CRITICAL_VITEST := \
	src/api/__tests__/client.spec.ts \
	src/api/__tests__/tokenRefresh.spec.ts \
	src/views/auth/__tests__/LinuxDoCallbackView.spec.ts \
	src/views/auth/__tests__/WechatCallbackView.spec.ts \
	src/views/user/__tests__/PaymentView.spec.ts \
	src/views/user/__tests__/PaymentResultView.spec.ts \
	src/components/user/profile/__tests__/ProfileInfoCard.spec.ts \
	src/views/admin/__tests__/SettingsView.spec.ts

FORK_BASE ?=

# 一键编译前后端
build: build-backend build-frontend

# 编译后端（复用 backend/Makefile）
build-backend:
	@$(MAKE) -C backend build

# 编译前端（需要已安装依赖）
build-frontend:
	@pnpm --dir frontend run build

# 编译 datamanagementd（宿主机数据管理进程）
build-datamanagementd:
	@cd datamanagement && go build -o datamanagementd ./cmd/datamanagementd

# 运行测试（后端 + 前端）
test: test-backend test-frontend

test-backend:
	@$(MAKE) -C backend test

test-frontend:
	@pnpm --dir frontend run lint:check
	@pnpm --dir frontend run typecheck
	@$(MAKE) test-frontend-critical

test-frontend-critical:
	@pnpm --dir frontend exec vitest run $(FRONTEND_CRITICAL_VITEST)

test-datamanagementd:
	@cd datamanagement && go test ./...

secret-scan:
	@python3 tools/secret_scan.py

fork-check:
	@tools/fork-maintenance/fork-maintenance.sh check-doc

fork-inventory:
	@tools/fork-maintenance/fork-maintenance.sh inventory $(if $(FORK_BASE),--base $(FORK_BASE),)

fork-snapshot:
	@tools/fork-maintenance/fork-maintenance.sh snapshot $(if $(FORK_BASE),--base $(FORK_BASE),)

fork-verify:
	@tools/fork-maintenance/fork-maintenance.sh verify-after-upstream

fork-restore-dry-run:
	@tools/fork-maintenance/fork-maintenance.sh reapply-production-state

deploy-gzip-dry-run:
	@deploy/local-gzip-binary-deploy.sh --skip-frontend-build --skip-backend-build
