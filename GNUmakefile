SHELL := bash

ACCTEST_PARALLELISM          ?= 20
ACCTEST_TIMEOUT              ?= 360m
GO_VER                       ?= $(shell echo go`cat .go-version | xargs`)
P                            ?= 20
PKG_NAME                     ?= internal
PROVIDER_VERSION             ?= $(shell cat version/VERSION | xargs)
PROVIDER_MIRROR_DIR          ?= terraform-plugin-dir/registry.terraform.io/ctyun-oos-lab/ctyunoos/$(PROVIDER_VERSION)
PROVIDER_RELEASE_DIR         ?= release
PROVIDER_BINARY_NAME         ?= terraform-provider-ctyunoos
PROVIDER_CHECKSUM_FILE       ?= $(PROVIDER_RELEASE_DIR)/$(PROVIDER_BINARY_NAME)_$(PROVIDER_VERSION)_SHA256SUMS
PROVIDER_MANIFEST_FILE       ?= $(PROVIDER_RELEASE_DIR)/$(PROVIDER_BINARY_NAME)_$(PROVIDER_VERSION)_manifest.json
SVC_DIR                      ?= ./internal/service
SWEEP                        ?= cn
SWEEP_DIR                    ?= ./internal/sweep
SWEEP_TIMEOUT                ?= 360m
TEST                         ?= ./...
TEST_COUNT                   ?= 1
TEST_OUTPUT                  ?=
TF_TEST_ROOT                 ?= examples
TF_TEST_DIR                  ?=
TF_TEST_CLI                  ?= terraform
TF_TEST_CLI_PATH             ?=
TF_TEST_RESOLVED_CLI_PATH     = $(if $(TF_TEST_CLI_PATH),$(TF_TEST_CLI_PATH),$(shell command -v "$(TF_TEST_CLI)" 2>/dev/null))
TF_TEST_INIT_ARGS            ?= -input=false
TF_TEST_ARGS                 ?=
TF_SHARED_BUCKET_DIR         ?= examples/compound/s3-shared-bucket
TF_SHARED_BUCKET_INIT_ARGS   ?= -input=false
TF_SHARED_BUCKET_APPLY_ARGS  ?=
TF_SHARED_BUCKET_TEST_ARGS   ?= -verbose
TF_SHARED_BUCKET_TEST_ROOT   ?= examples
TF_PLUGIN_CACHE_DIR           ?= $(CURDIR)/.terraform-plugin-cache
WITH_SHARED_BUCKET            ?=
# Minimum CLI version (major.minor) required to run example (.tftest.hcl) tests.
# The modern terraform test framework is only available from 1.6 onward, so
# older CLIs (e.g. terraform 1.5.7) skip example tests instead of failing.
TF_TEST_MIN_VERSION           ?= 1.6
TENV                          ?= tenv
TENV_ROOT                     ?= $(HOME)/.tenv
TENV_INSTALL_ARGS             ?=
TENV_COMPATIBILITY_MATRIX     ?=
COMPATIBILITY_TESTS           ?= ^TestAcc

# terraform-plugin-testing uses this path for the CLI that drives acceptance tests.
# Resolved from TF_TEST_CLI (default terraform); select OpenTofu explicitly
# with `make testacc TF_TEST_CLI=tofu`.
TF_ACC_TERRAFORM_PATH         ?= $(shell command -v "$(TF_TEST_CLI)" 2>/dev/null)
TF_ACC_PROVIDER_HOST           ?= $(if $(filter tofu,$(TF_TEST_CLI) $(notdir $(TF_ACC_TERRAFORM_PATH))),registry.opentofu.org)
BUILD_ALL_PLATFORMS          ?= darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64

export TF_PLUGIN_CACHE_DIR

# NOTE: Keep targets in alphabetical order

# VARIABLE REFERENCE:
# Service-specific variables (interchangeable for user convenience):
#   PKG=<service>     - Service name (e.g., ses, lambda, s3) - traditional usage
#   K=<service>       - Service name (e.g., ses, lambda, s3) - shorter alias
#
# Test-specific variables:
#   T=<pattern>       - Test name pattern (e.g., TestAccLambda) - preferred
#   TESTS=<pattern>   - Test name pattern - legacy alias for T
#   TEST_OUTPUT=<prefix> - Save test output as <prefix>-YYYYMMDD-HHMMSS.log
#   WITH_SHARED_BUCKET=1        - Run shared-bucket manual tests with testiac
#   TENV_COMPATIBILITY_MATRIX - Ordered CLI/version pairs for testcompat,
#                               e.g. "terraform:1.12.0 tofu:1.12.6"
#   TENV_INSTALL_ARGS           - Extra flags passed to tenv install (e.g. -a amd64)
#   COMPATIBILITY_TESTS         - ACC test pattern for testcompat (default: ^TestAcc)
#
# Derived variables (set automatically based on above):
#   PKG_NAME          - Full package path (e.g., internal/service/ses)
#   SVC_DIR           - Service directory path (e.g., ./internal/service/ses)
#   TEST              - Test path pattern (e.g., ./internal/service/ses/...)
#
# Examples:
#   make quick-fix PKG=s3      # Fix code in S3 service
#   make quick-fix K=iam       # Same as above, but shorter (both work)
#   make testacc T=TestAccIAMRole PKG=iam  # Run specific test in IAM service
#   make testiac                           # Run standard IaC tests
#   make testiac WITH_SHARED_BUCKET=1      # Run IaC tests + shared-bucket manual tests
#   make testcompat TENV_COMPATIBILITY_MATRIX="terraform:1.12.0 tofu:1.12.6"

# Variable consolidation for backward compatibility and user convenience:
# - PKG and K both refer to service names (e.g., 'ses', 'lambda')
# - If one is provided, automatically set the other for consistency
# - This allows 'make quick-fix PKG=ses' and 'make quick-fix K=ses' to work identically
ifneq ($(origin PKG), undefined)
	PKG_NAME = internal/service/$(PKG)
	SVC_DIR = ./internal/service/$(PKG)
	TEST = ./$(PKG_NAME)/...
	# Auto-set K for compatibility
	K = $(PKG)
endif

ifneq ($(origin K), undefined)
	PKG_NAME = internal/service/$(K)
	SVC_DIR = ./internal/service/$(K)
	TEST = ./$(PKG_NAME)/...
	# Auto-set PKG for compatibility (only if not already set)
	ifeq ($(origin PKG), undefined)
		PKG = $(K)
	endif
endif

ifneq ($(origin TESTS), undefined)
	RUNARGS = -run='$(TESTS)'
endif

ifneq ($(origin T), undefined)
	RUNARGS = -run='$(T)'
endif

TESTACC_RUNARGS = -run='^TestAcc'
ifneq ($(strip $(TESTS)),)
TESTACC_RUNARGS = -run='$(TESTS)'
endif
ifneq ($(strip $(T)),)
TESTACC_RUNARGS = -run='$(T)'
endif

ifneq ($(origin SWEEPERS), undefined)
	SWEEPARGS = -sweep-run='$(SWEEPERS)'
endif

ifeq ($(origin CURDIR), undefined)
	CURDIR = $(PWD)
endif

ifneq ($(P), 20)
	ACCTEST_PARALLELISM = $(P)
endif

default: build ## build

build: prereq-go fmt-check ## Build provider
	@echo "make: Building provider binary..."
	$(GO_VER) install .

release: prereq-go ## Build provider binaries and standard ZIP packages for all platforms
	@echo "make: Building provider binaries and ZIP packages for all supported platforms..."
	@package_tool=""; \
	if command -v zip >/dev/null 2>&1; then package_tool="zip"; \
	elif command -v bsdtar >/dev/null 2>&1; then package_tool="bsdtar"; \
	else echo "make: zip or bsdtar command is required to package provider releases" >&2; exit 1; fi ; \
	echo "make: Packaging with $$package_tool" ; \
	command -v sha256sum >/dev/null 2>&1 || command -v shasum >/dev/null 2>&1 || { echo "make: sha256sum or shasum is required to create release checksums" >&2; exit 1; } ; \
	mkdir -p "$(PROVIDER_RELEASE_DIR)" ; \
	staging_dir="$(PROVIDER_RELEASE_DIR)/.staging" ; \
	trap 'rm -rf "$$staging_dir"' EXIT ; \
	: > "$(PROVIDER_CHECKSUM_FILE)" ; \
	if [ -f terraform-registry-manifest.json ]; then cp terraform-registry-manifest.json "$(PROVIDER_MANIFEST_FILE)"; else echo "make: terraform-registry-manifest.json is required" >&2; exit 1; fi ; \
	for platform in $(BUILD_ALL_PLATFORMS); do \
		goos=$${platform%/*}; \
		goarch=$${platform#*/}; \
		os_arch=$${goos}_$${goarch}; \
		ext="" ; \
		if [ "$$goos" = "windows" ]; then ext=".exe"; fi ; \
		package_dir="$(PROVIDER_RELEASE_DIR)/.staging/$$os_arch" ; \
		package_bin="$$package_dir/$(PROVIDER_BINARY_NAME)_v$(PROVIDER_VERSION)$$ext" ; \
		package="$(PROVIDER_RELEASE_DIR)/$(PROVIDER_BINARY_NAME)_$(PROVIDER_VERSION)_$$os_arch.zip" ; \
		echo "make: Building $$os_arch -> $$package_bin" ; \
		mkdir -p "$$package_dir" ; \
		CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch $(GO_VER) build -o "$$package_bin" . || exit 1 ; \
		if [ "$$package_tool" = "zip" ]; then \
			(cd "$$package_dir" && zip -X -q -j "$(CURDIR)/$$package" "$$(basename "$$package_bin")") || exit 1 ; \
		else \
			(cd "$$package_dir" && bsdtar -a -cf "$(CURDIR)/$$package" "$$(basename "$$package_bin")") || exit 1 ; \
		fi ; \
		package_filename="$$(basename "$$package")" ; \
		if command -v sha256sum >/dev/null 2>&1; then (cd "$(PROVIDER_RELEASE_DIR)" && sha256sum "$$package_filename"); else (cd "$(PROVIDER_RELEASE_DIR)" && shasum -a 256 "$$package_filename"); fi >> "$(PROVIDER_CHECKSUM_FILE)" ; \
		echo "make: Packaged $$package" ; \
	done ; \
	manifest_filename="$$(basename "$(PROVIDER_MANIFEST_FILE)")" ; \
	if command -v sha256sum >/dev/null 2>&1; then (cd "$(PROVIDER_RELEASE_DIR)" && sha256sum "$$manifest_filename"); else (cd "$(PROVIDER_RELEASE_DIR)" && shasum -a 256 "$$manifest_filename"); fi >> "$(PROVIDER_CHECKSUM_FILE)" ; \
	echo "make: Wrote checksums to $(PROVIDER_CHECKSUM_FILE)"

ci: tools go-build gen-check docs-lint docs-misspell testacc-lint deps-check go-misspell golangci-lint import-lint sweeper-check test ## [CI] Run all OOS CI checks

clean: clean-ci clean-make-tests clean-terraform-examples clean-go clean-tidy build ## Clean up Go cache, tidy and rebuild
	@echo "make: Clean complete"

clean-ci: ## Clean up CI tool artifacts
	@echo "make: Cleaning CI tool artifacts..."
	@rm -rf .ci/markdownlint/node_modules

clean-go: prereq-go ## Clean up Go cache
	@echo "make: Cleaning Go..."
	@echo "make: WARNING: This will kill gopls and clean Go caches"
	@vscode=`ps -ef | grep Visual\ Studio\ Code | wc -l | xargs` ; \
	if [ $$vscode -gt 1 ] ; then \
		echo "make: ALERT: vscode is running. Close it and try again." ; \
		exit 1 ; \
	fi
	@for proc in `pgrep gopls` ; do \
		echo "make: killing gopls process $$proc" ; \
		kill -9 $$proc ; \
	done ; \
	echo "make: cleaning Go caches..." ; \
	$(GO_VER) clean -modcache -testcache -cache -i -r
	go clean -modcache -testcache -cache -i -r
	@echo "make: Go caches cleaned"

clean-make-tests: ## Clean up artifacts from make tests
	@echo "make: Cleaning up artifacts from make tests..."
	@rm -f docscheck
	@rm -rf sweeper-bin
	@rm -rf terraform-plugin-dir
	@rm -rf .terraform/providers
	@rm -rf .terraform-plugin-cache

clean-terraform-examples: ## Remove Terraform work directories and lock files under examples
	@set -e; \
	echo "make: Removing Terraform work directories and lock files under examples/..."; \
	find examples -type d -name .terraform -prune -exec rm -rf {} +; \
	find examples -type f -name .terraform.lock.hcl -delete

clean-tidy: prereq-go ## Clean up tidy
	@echo "make: Tidying Go mods..."
	@gover="$(GO_VER)" ; \
	echo "make: tidying with $$gover" ; \
	if [ "$$gover" = "go" ] ; then \
		gover=go`cat .go-version | xargs` ; \
		echo "make: WARNING: no version provided so tidying with $$gover" ; \
		echo "make: tidying with newer versions can make go.mod incompatible" ; \
		echo "make: to use a different version, use 'GO_VER=go1.16 make clean-tidy'" ; \
		echo "make: to use the version in .go-version, use 'make clean-tidy'" ; \
		echo "make: if you get an error, see https://go.dev/doc/manage-install to locally install various Go versions" ; \
	fi ; \
	cd .ci/tools && $$gover mod tidy && cd ../.. ; \
	$$gover mod tidy
	@echo "make: Go mods tidied"

# testcompat delegates ACC execution to testacc so terraform-plugin-testing owns temporary working directories and lifecycle.
testcompat: ## Run Go ACC and examples Terraform tests for an ordered tenv CLI/version matrix
	@set -eu; \
	if ! command -v "$(TENV)" >/dev/null 2>&1; then \
		echo "make: tenv not found; install tenv or set TENV=/path/to/tenv" >&2; \
		exit 1; \
	fi; \
	if [ -z "$(strip $(TENV_COMPATIBILITY_MATRIX))" ]; then \
		echo "make: TENV_COMPATIBILITY_MATRIX is required" >&2; \
		echo "make: example: make testcompat TENV_COMPATIBILITY_MATRIX=\"terraform:1.12.0 tofu:1.12.6\"" >&2; \
		exit 1; \
	fi; \
	overall_status=0; \
	for pair in $(TENV_COMPATIBILITY_MATRIX); do \
		cli="$${pair%%:*}"; \
		version="$${pair#*:}"; \
		if [ -z "$$version" ] || [ "$$version" = "$$pair" ]; then \
			echo "make: invalid compatibility matrix entry '$$pair'; expected terraform:<version> or tofu:<version>" >&2; \
			exit 1; \
		fi; \
		case "$$cli" in \
			terraform) tenv_tool=tf; tenv_product=Terraform; binary=terraform; provider_host=registry.terraform.io ;; \
			tofu) tenv_tool=tofu; tenv_product=OpenTofu; binary=tofu; provider_host=registry.opentofu.org ;; \
			*) echo "make: unsupported compatibility CLI '$$cli' in '$$pair'; use terraform or tofu" >&2; exit 1 ;; \
		esac; \
		echo "make: Installing $$tenv_product $$version with tenv..."; \
		"$(TENV)" --root-path "$(TENV_ROOT)" "$$tenv_tool" install "$$version" $(TENV_INSTALL_ARGS); \
		cli_path="$(TENV_ROOT)/$$tenv_product/$$version/$$binary"; \
		if [ ! -x "$$cli_path" ]; then \
			echo "make: tenv installed $$tenv_product $$version but CLI is missing: $$cli_path" >&2; \
			exit 1; \
		fi; \
		echo "make: Testing $$cli $$version with $$cli_path"; \
		compat_log_prefix="$(TEST_OUTPUT)"; \
		if [ -n "$$compat_log_prefix" ]; then compat_log_prefix="$${compat_log_prefix}-$$cli-$$version"; fi; \
		compat_acc_args=""; compat_examples_args=""; \
		if [ -n "$$compat_log_prefix" ]; then compat_acc_args="TEST_OUTPUT=$$compat_log_prefix-acc"; compat_examples_args="TEST_OUTPUT=$$compat_log_prefix-examples"; fi; \
		testacc_status=0; \
		testiac_status=0; \
		testiac_log=$$(mktemp); \
		$(MAKE) --no-print-directory testacc \
			TF_TEST_CLI="$$cli" TF_ACC_TERRAFORM_PATH="$$cli_path" \
			TESTS="$(COMPATIBILITY_TESTS)" T= TESTARGS= \
			$$compat_acc_args \
			TF_ACC_PROVIDER_HOST="$$provider_host" || testacc_status=$$?; \
		$(MAKE) --no-print-directory testiac \
			TF_TEST_CLI="$$cli" TF_TEST_CLI_PATH="$$cli_path" \
			$$compat_examples_args 2>&1 | tee "$$testiac_log" || testiac_status=$${PIPESTATUS[0]}; \
		testiac_skipped=0; \
		if grep -q "make: testiac result: skipped" "$$testiac_log"; then testiac_skipped=1; fi; \
		rm -f "$$testiac_log"; \
		if [ $$testacc_status -ne 0 ] || [ $$testiac_status -ne 0 ]; then \
			echo "make: testcompat FAILED for $$cli $$version (testacc: $$testacc_status, testiac: $$testiac_status)"; \
			overall_status=1; \
		elif [ $$testiac_skipped -eq 1 ]; then \
			echo "make: testcompat passed for $$cli $$version (examples skipped: CLI below $(TF_TEST_MIN_VERSION))"; \
		else \
			echo "make: testcompat passed for $$cli $$version"; \
		fi; \
	done; \
	if [ $$overall_status -ne 0 ]; then \
		echo "make: testcompat completed with failures"; \
		exit 1; \
	fi; \
	echo "make: testcompat completed successfully"

deps-check: prereq-go ## [CI] Dependency Checks / go_mod
	@echo "make: Dependency Checks / go_mod..."
	@cd .ci/tools && $(GO_VER) mod tidy -diff
	@$(GO_VER) mod tidy -diff

docs-lint: prereq-go ## [CI] Documentation Checks / markdown lint
	@echo "make: Documentation Checks / markdown lint..."
	@$(GO_VER) run ./internal/generate/docscheck docs

docs-misspell: ## [CI] Documentation Checks / misspell
	@echo "make: Documentation Checks / misspell..."
	@misspell -error -source text docs

fix-imports: ## Fixing source code imports with goimports
	@echo "make: Fixing source code imports with goimports..."
	@if [ -d "./$(PKG_NAME)" ] && ls ./$(PKG_NAME)/*.go >/dev/null 2>&1; then \
		echo "make: Processing ./$(PKG_NAME)..."; \
		goimports -w ./$(PKG_NAME)/*.go; \
	fi
	@for dir in $$(find ./$(PKG_NAME) -mindepth 1 -type d | sort); do \
		if ls $$dir/*.go >/dev/null 2>&1; then \
			echo "make: Processing $$dir..."; \
			goimports -w $$dir/*.go; \
		fi; \
	done

fmt: ## Fix Go source formatting
	@echo "make: Fixing source code with gofmt..."
	@find . -name '*.go' | grep -v vendor | xargs gofmt -s -w

# Currently required by tf-deploy compile
fmt-check: ## Verify Go source is formatted
	@echo "make: Verifying source code with gofmt..."
	@sh -c "'$(CURDIR)/.ci/scripts/gofmtcheck.sh'"

gen: prereq-go gen-raw ## Run all Go generators (with Go version check)

gen-raw: ## Run all Go generators
	@echo "make: Running Go generators..."
	$(GO_VER) generate ./...
	# Generate service package lists last as they may depend on output of earlier generators.
	cd internal/provider/sdkv2 && GOPACKAGE=sdkv2 $(GO_VER) run ../../generate/servicepackages/main.go -ServicePackageRoot ../../service
	cd internal/sweep && GOPACKAGE=sweep_test $(GO_VER) run ../generate/servicepackages/main.go -ServicePackageRoot ../service -- service_packages_gen_test.go

gen-check: gen ## [CI] Provider Checks / go_generate
	@echo "make: Provider Checks / go_generate..."
	@echo "make: Checking if 'make gen' produces uncommitted changes..."
	@git diff --compact-summary --exit-code || \
		(echo; echo "Unexpected difference after code generation. Run 'make gen' and commit the changes."; exit 1)

go-build: prereq-go ## [CI] Provider Checks / go-build
	@os_arch=`$(GO_VER) env GOOS`_`$(GO_VER) env GOARCH` ; \
	echo "make: Provider Checks / go-build ($$os_arch)..." ; \
	out="$(PROVIDER_MIRROR_DIR)/$$os_arch/terraform-provider-ctyunoos" ; \
	mkdir -p "$$(dirname "$$out")" ; \
	$(GO_VER) build -o "$$out" .

go-misspell: ## [CI] Provider Checks / misspell
	@echo "make: Provider Checks / misspell..."
	@misspell -error -source auto -i "littel,ceasar,ect" internal/

golangci-lint: ## [CI] golangci-lint Checks
	@echo "make: golangci-lint Checks..."
	@golangci-lint run \
		--config .ci/.golangci.yml \
		$(TEST)

help: ## Display this help
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-27s\033[0m %s\n", $$1, $$2}'

import-lint: ## [CI] Provider Checks / import-lint
	@echo "make: Provider Checks / import-lint..."
	@impi --local . --scheme stdThirdPartyLocal $(TEST)

lint: golangci-lint import-lint ## Run OOS Go lint checks

prereq-go: ## If $(GO_VER) is not installed, install it
	@if ! type "$(GO_VER)" > /dev/null 2>&1 ; then \
		echo "make: $(GO_VER) not found" ; \
		echo "make: installing $(GO_VER)..." ; \
		echo "make: if you get an error, see https://go.dev/doc/manage-install to locally install various Go versions" ; \
		go install golang.org/dl/$(GO_VER)@latest ; \
		$(GO_VER) download ; \
		echo "make: $(GO_VER) ready" ; \
	fi

quick-fix: fmt testacc-lint-fix fix-imports terraform-fmt ## Some quick fixes

sweep: prereq-go ## Run sweepers
	# make sweep SWEEPARGS=-sweep-run=ctyunoos_example_thing
	# set SWEEPARGS=-sweep-allow-failures to continue after first failure
	@echo "WARNING: This will destroy infrastructure. Use only in development accounts."
	$(GO_VER) test $(SWEEP_DIR) -v -sweep=$(SWEEP) $(SWEEPARGS) -timeout $(SWEEP_TIMEOUT) -vet=off

sweeper-check: sweeper-linked sweeper-unlinked ## [CI] Provider Checks / Sweeper Linked, Unlinked

sweeper-linked: ## [CI] Provider Checks / Sweeper Functions Linked
	@echo "make: Provider Checks / Sweeper Functions Linked..." ; \
	go test -c -o ./sweeper-bin ./internal/sweep/ ; \
	count=`strings ./sweeper-bin | \
		grep --count --extended-regexp 'internal/service/[a-zA-Z0-9]+\.sweep[a-zA-Z0-9]+$$'` ; \
	echo "make: sweeper-linked: found $$count, expected more than 0" ; \
	[ $$count -gt 0 ] || \
		(echo; echo "Expected `strings` to detect sweeper function names in sweeper binary."; exit 1)

sweeper-unlinked: go-build ## [CI] Provider Checks / Sweeper Functions Not Linked
	@os_arch=`go env GOOS`_`go env GOARCH` ; \
	echo "make: Provider Checks / Sweeper Functions Not Linked ($$os_arch)..." ; \
	count=`strings "$(PROVIDER_MIRROR_DIR)/$$os_arch/terraform-provider-ctyunoos" | \
		grep --count --extended-regexp 'internal/service/[a-zA-Z0-9]+\.sweep[a-zA-Z0-9]+$$'` ; \
	echo "make: sweeper-unlinked: found $$count, expected 0" ; \
	[ $$count -eq 0 ] || \
		(echo "Expected `strings` to detect no sweeper function names in provider binary."; exit 1)

test: prereq-go ## Run unit tests (auto-detects environment and scope)
	@branch=$$(git rev-parse --abbrev-ref HEAD); \
	printf "make: Running unit tests on branch: \033[1m%s\033[0m...\n" "[ $$branch ]"
	@# Auto-detect: single service or full codebase
	@if [ -n "$(PKG)$(K)" ]; then \
		echo "Testing single service: $(or $(PKG),$(K))"; \
		$(MAKE) test-single-service; \
	else \
		echo "Testing full codebase"; \
		$(MAKE) test-full; \
	fi

test-single-service: ## Internal: test single service
	@# macOS: use temp cache to avoid CrowdStrike scanning
	@if [ "$$(uname)" = "Darwin" ]; then \
		build_dir="/tmp/terraform-$(or $(PKG),$(K))-$$$$"; \
		mkdir -p "$$build_dir/cache"; \
		export GOCACHE="$$build_dir/cache"; \
		export GOTMPDIR="$$build_dir"; \
	fi; \
	cores=$$(getconf _NPROCESSORS_ONLN 2>/dev/null || nproc 2>/dev/null || echo 8); \
	test_parallel=$${TEST_PARALLEL:-$$cores}; \
	printf "Testing with -parallel %s\n" "$$test_parallel"; \
	test_output="$(TEST_OUTPUT)"; \
	if [ -n "$$test_output" ]; then \
		test_output="$${test_output}-$$(date +%Y%m%d-%H%M%S).log"; \
		mkdir -p "$$(dirname "$$test_output")"; \
		echo "make: Saving test output to $$test_output"; \
		$(GO_VER) test $(TEST) -parallel $$test_parallel -run '^Test[^A]|^TestA[^c]|^TestAc[^c]' $(RUNARGS) $(TESTARGS) -timeout 30m -vet=off -buildvcs=false -count=1 2>&1 | tee "$$test_output"; \
		pipeline_status=($${PIPESTATUS[@]}); \
		test_status=$${pipeline_status[0]}; \
		if [ "$$test_status" -eq 0 ]; then test_status=$${pipeline_status[1]}; fi; \
	else \
		$(GO_VER) test $(TEST) -parallel $$test_parallel -run '^Test[^A]|^TestA[^c]|^TestAc[^c]' $(RUNARGS) $(TESTARGS) -timeout 30m -vet=off -buildvcs=false -count=1; \
		test_status=$$?; \
	fi; \
	if [ "$$(uname)" = "Darwin" ] && [ -n "$$build_dir" ]; then rm -rf "$$build_dir"; fi; \
	exit $$test_status

test-full: ## Internal: test full codebase
	@# macOS: use temp cache to avoid CrowdStrike scanning
	@if [ "$$(uname)" = "Darwin" ]; then \
		build_dir="/tmp/terraform-oos-build-$$$$"; \
		mkdir -p "$$build_dir/cache" "$$build_dir/tmp"; \
		export GOCACHE="$$build_dir/cache"; \
		export GOTMPDIR="$$build_dir/tmp"; \
	fi; \
	cores=$$(getconf _NPROCESSORS_ONLN 2>/dev/null || nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 8); \
	test_p=$${TEST_P:-$$cores}; \
	test_parallel=$${TEST_PARALLEL:-$$((cores * 2))}; \
	printf "Testing with -p %s -parallel %s\n" "$$test_p" "$$test_parallel"; \
	test_output="$(TEST_OUTPUT)"; \
	if [ -n "$$test_output" ]; then \
		test_output="$${test_output}-$$(date +%Y%m%d-%H%M%S).log"; \
		mkdir -p "$$(dirname "$$test_output")"; \
		echo "make: Saving test output to $$test_output"; \
		$(GO_VER) test $(TEST) -p $$test_p -parallel $$test_parallel -run '^Test[^A]|^TestA[^c]|^TestAc[^c]' $(RUNARGS) $(TESTARGS) -timeout 60m -vet=off -buildvcs=false 2>&1 | tee "$$test_output"; \
		pipeline_status=($${PIPESTATUS[@]}); \
		test_status=$${pipeline_status[0]}; \
		if [ "$$test_status" -eq 0 ]; then test_status=$${pipeline_status[1]}; fi; \
	else \
		$(GO_VER) test $(TEST) -p $$test_p -parallel $$test_parallel -run '^Test[^A]|^TestA[^c]|^TestAc[^c]' $(RUNARGS) $(TESTARGS) -timeout 60m -vet=off -buildvcs=false; \
		test_status=$$?; \
	fi; \
	if [ "$$(uname)" = "Darwin" ] && [ -n "$$build_dir" ]; then rm -rf "$$build_dir"; fi; \
	exit $$test_status

testacc: prereq-go fmt-check ## Run acceptance tests (all TestAcc* under internal/ by default; use PKG/TESTS to narrow)
	@branch=$$(git rev-parse --abbrev-ref HEAD); \
	printf "make: Running acceptance tests on branch: \033[1m%s\033[0m...\n" "[ $$branch ]"
	@if [ -n "$(TF_ACC_TERRAFORM_PATH)" ] && [ ! -x "$(TF_ACC_TERRAFORM_PATH)" ]; then \
		echo "make: acceptance CLI not found or not executable: $(TF_ACC_TERRAFORM_PATH)" >&2; \
		echo "make: set TF_TEST_CLI=terraform|tofu or TF_ACC_TERRAFORM_PATH=/path/to/cli" >&2; \
		exit 1; \
	fi
	@if [ -n "$(TF_ACC_TERRAFORM_PATH)" ]; then echo "make: Acceptance CLI: $(TF_ACC_TERRAFORM_PATH)"; else echo "make: Acceptance CLI: terraform-plugin-testing default resolution"; fi
	@if [ -n "$(TF_ACC_PROVIDER_HOST)" ]; then echo "make: Acceptance provider host: $(TF_ACC_PROVIDER_HOST)"; fi
	@mkdir -p "$(TF_PLUGIN_CACHE_DIR)"
	@if [ "$(TESTARGS)" = "-run=TestAccXXX" ]; then \
		echo ""; \
		echo "Error: Skipping example acceptance testing pattern. Update PKG and TESTS for the relevant *_test.go file."; \
		echo ""; \
		echo "For example if updating internal/service/iam/role.go, use the test names in internal/service/iam/role_test.go starting with TestAcc and up to the underscore:"; \
		echo "make testacc TESTS=TestAccIAMRole_ PKG=iam"; \
		echo ""; \
		echo "See the relevant *_test.go file for TestAcc test names to target."; \
		 exit 1; \
	fi
	@if [ -n "$(TEST_OUTPUT)" ]; then \
		test_output="$(TEST_OUTPUT)-$$(date +%Y%m%d-%H%M%S).log"; \
		mkdir -p "$$(dirname "$$test_output")"; \
		echo "make: Saving test output to $$test_output"; \
		TF_ACC=1 $(if $(TF_ACC_TERRAFORM_PATH),TF_ACC_TERRAFORM_PATH="$(TF_ACC_TERRAFORM_PATH)") $(if $(TF_ACC_PROVIDER_HOST),TF_ACC_PROVIDER_HOST="$(TF_ACC_PROVIDER_HOST)") $(GO_VER) test ./$(PKG_NAME)/... -v -count $(TEST_COUNT) -p 1 -parallel $(ACCTEST_PARALLELISM) $(TESTACC_RUNARGS) $(TESTARGS) -timeout $(ACCTEST_TIMEOUT) -vet=off 2>&1 | tee "$$test_output"; \
		pipeline_status=($${PIPESTATUS[@]}); \
		if [ "$${pipeline_status[0]}" -ne 0 ]; then exit "$${pipeline_status[0]}"; fi; \
		exit "$${pipeline_status[1]}"; \
	else \
		TF_ACC=1 $(if $(TF_ACC_TERRAFORM_PATH),TF_ACC_TERRAFORM_PATH="$(TF_ACC_TERRAFORM_PATH)") $(if $(TF_ACC_PROVIDER_HOST),TF_ACC_PROVIDER_HOST="$(TF_ACC_PROVIDER_HOST)") $(GO_VER) test ./$(PKG_NAME)/... -v -count $(TEST_COUNT) -p 1 -parallel $(ACCTEST_PARALLELISM) $(TESTACC_RUNARGS) $(TESTARGS) -timeout $(ACCTEST_TIMEOUT) -vet=off; \
	fi

testacc-lint: ## [CI] Acceptance Test Linting / terrafmt
	@echo "make: Acceptance Test Linting / terrafmt..."
	@find $(SVC_DIR) -type f -name '*_test.go' \
		| sort -u \
		| xargs -I {} terrafmt diff --check --fmtcompat {}

testacc-lint-fix: ## Fix acceptance test linter findings
	@echo "make: Fixing Acceptance Test Linting / terrafmt..."
	@find $(SVC_DIR) -type f -name '*_test.go' \
		| sort -u \
		| xargs -I {} terrafmt fmt  --fmtcompat {}

terraform-fmt: ## Format all .tf, .tfvars, .tftest.hcl, and .tfquery.hcl files
	@echo "make: Formatting .tf, .tfvars, .tftest.hcl, and .tfquery.hcl files..."
	@find . -name "*.tfquery.hcl" -type f -exec sh -c 'mv "$$1" "$${1%.tfquery.hcl}.BEGIANT.tf"' _ {} \;
	@terraform fmt -recursive .
	@find . -name "*.BEGIANT.tf" -type f -exec sh -c 'mv "$$1" "$${1%.BEGIANT.tf}.tfquery.hcl"' _ {} \;

shared-bucket-create: ## Create the persistent shared S3 bucket used by manual object tests
	@set -e; \
	mkdir -p "$(TF_PLUGIN_CACHE_DIR)"; \
	"$(TF_TEST_RESOLVED_CLI_PATH)" -chdir="$(TF_SHARED_BUCKET_DIR)" init $(TF_SHARED_BUCKET_INIT_ARGS); \
	"$(TF_TEST_RESOLVED_CLI_PATH)" -chdir="$(TF_SHARED_BUCKET_DIR)" apply -auto-approve $(TF_SHARED_BUCKET_APPLY_ARGS)

shared-bucket-destroy: ## Destroy the persistent shared S3 bucket after its OOS object retention period expires
	@set -e; \
	mkdir -p "$(TF_PLUGIN_CACHE_DIR)"; \
	"$(TF_TEST_RESOLVED_CLI_PATH)" -chdir="$(TF_SHARED_BUCKET_DIR)" init $(TF_SHARED_BUCKET_INIT_ARGS); \
	"$(TF_TEST_RESOLVED_CLI_PATH)" -chdir="$(TF_SHARED_BUCKET_DIR)" destroy -auto-approve $(TF_SHARED_BUCKET_APPLY_ARGS)

shared-bucket-test: ## Run all shared-bucket manual tests, or one selected by TF_TEST_DIR
	@set -e; \
	cli_ver=$$("$(TF_TEST_RESOLVED_CLI_PATH)" version | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1); \
	req_ver="$(TF_TEST_MIN_VERSION)"; \
	maj=$${cli_ver%%.*}; rest=$${cli_ver#*.}; min=$${rest%%.*}; \
	req_maj=$${req_ver%%.*}; req_min=$${req_ver#*.}; \
	if [ "$$maj" -lt "$$req_maj" ] || { [ "$$maj" -eq "$$req_maj" ] && [ "$$min" -lt "$$req_min" ]; }; then \
		echo "make: $(TF_TEST_CLI) v$$cli_ver < $$req_ver; skipping shared-bucket example tests"; \
		exit 0; \
	fi; \
	mkdir -p "$(TF_PLUGIN_CACHE_DIR)"; \
	test_output="$(TEST_OUTPUT)"; \
	if [ -n "$$test_output" ]; then \
		test_output="$${test_output}-$$(date +%Y%m%d-%H%M%S).log"; \
		mkdir -p "$$(dirname "$$test_output")"; \
		echo "make: Saving Terraform test output to $$test_output"; \
		run_logged() { \
			"$$@" 2>&1 | tee -a "$$test_output"; \
			pipeline_status=($${PIPESTATUS[@]}); \
			if [ "$${pipeline_status[0]}" -ne 0 ]; then return "$${pipeline_status[0]}"; fi; \
			return "$${pipeline_status[1]}"; \
		}; \
	fi; \
	bucket_name=$$("$(TF_TEST_RESOLVED_CLI_PATH)" -chdir="$(TF_SHARED_BUCKET_DIR)" output -raw bucket_name); \
	test_dirs="$(TF_TEST_DIR)"; \
	if [ -z "$$test_dirs" ]; then \
		if [ ! -d "$(TF_SHARED_BUCKET_TEST_ROOT)" ]; then \
			echo "No shared-bucket test root found: $(TF_SHARED_BUCKET_TEST_ROOT)" >&2; \
			exit 1; \
		fi; \
		test_dirs=$$(find "$(TF_SHARED_BUCKET_TEST_ROOT)" -type f -path '*/manual-tests/*.tftest.hcl' -print \
			| sed 's:/manual-tests/[^/]*$$::' \
			| sort -u); \
	fi; \
	if [ -z "$$test_dirs" ]; then \
		echo "No manual Terraform test files found under $(TF_SHARED_BUCKET_TEST_ROOT)" >&2; \
		exit 1; \
	fi; \
	for test_dir in $$test_dirs; do \
		if [ ! -d "$$test_dir" ] && [ -d "$(TF_SHARED_BUCKET_TEST_ROOT)/$$test_dir" ]; then test_dir="$(TF_SHARED_BUCKET_TEST_ROOT)/$$test_dir"; fi; \
		if [ ! -d "$$test_dir/manual-tests" ]; then \
			echo "Shared-bucket manual test directory does not exist: $$test_dir/manual-tests" >&2; \
			exit 1; \
		fi; \
		echo "make: Cleaning $$test_dir/.terraform and lock file before testing"; \
		rm -rf "$$test_dir/.terraform" "$$test_dir/.terraform.lock.hcl"; \
		echo "make: Running shared-bucket $(TF_TEST_CLI) acceptance test in $$test_dir"; \
		if [ -n "$$test_output" ]; then \
			run_logged "$(TF_TEST_RESOLVED_CLI_PATH)" -chdir="$$test_dir" init $(TF_TEST_INIT_ARGS); \
			run_logged "$(TF_TEST_RESOLVED_CLI_PATH)" -chdir="$$test_dir" test -test-directory=manual-tests -var="bucket_name=$$bucket_name" $(TF_SHARED_BUCKET_TEST_ARGS); \
		else \
			"$(TF_TEST_RESOLVED_CLI_PATH)" -chdir="$$test_dir" init $(TF_TEST_INIT_ARGS); \
			"$(TF_TEST_RESOLVED_CLI_PATH)" -chdir="$$test_dir" test -test-directory=manual-tests -var="bucket_name=$$bucket_name" $(TF_SHARED_BUCKET_TEST_ARGS); \
		fi; \
	done

testiac: ## Run all IaC CLI acceptance tests under TF_TEST_ROOT; override with TF_TEST_CLI=tofu
	@if [ -z "$(TF_TEST_RESOLVED_CLI_PATH)" ] || [ ! -x "$(TF_TEST_RESOLVED_CLI_PATH)" ]; then \
		echo "make: test CLI not found or not executable: $(TF_TEST_RESOLVED_CLI_PATH)" >&2; \
		echo "make: set TF_TEST_CLI=terraform|tofu or TF_TEST_CLI_PATH=/path/to/cli" >&2; \
		exit 1; \
	fi
	@set -e; \
	cli_ver=$$("$(TF_TEST_RESOLVED_CLI_PATH)" version | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1); \
	req_ver="$(TF_TEST_MIN_VERSION)"; \
	maj=$${cli_ver%%.*}; rest=$${cli_ver#*.}; min=$${rest%%.*}; \
	req_maj=$${req_ver%%.*}; req_min=$${req_ver#*.}; \
	if [ "$$maj" -lt "$$req_maj" ] || { [ "$$maj" -eq "$$req_maj" ] && [ "$$min" -lt "$$req_min" ]; }; then \
		echo "make: $(TF_TEST_CLI) v$$cli_ver < $$req_ver; skipping example (.tftest.hcl) tests"; \
		echo "make: testiac result: skipped (CLI below $$req_ver)"; \
		exit 0; \
	fi; \
	mkdir -p "$(TF_PLUGIN_CACHE_DIR)"; \
	test_output="$(TEST_OUTPUT)"; \
	if [ -n "$$test_output" ]; then \
		test_output="$${test_output}-$$(date +%Y%m%d-%H%M%S).log"; \
		mkdir -p "$$(dirname "$$test_output")"; \
		echo "make: Saving $(TF_TEST_CLI) test output to $$test_output"; \
		run_logged() { \
			"$$@" 2>&1 | tee -a "$$test_output"; \
			pipeline_status=($${PIPESTATUS[@]}); \
			if [ "$${pipeline_status[0]}" -ne 0 ]; then return "$${pipeline_status[0]}"; fi; \
			return "$${pipeline_status[1]}"; \
		}; \
	fi; \
	test_dirs="$(TF_TEST_DIR)"; \
	if [ -z "$$test_dirs" ]; then \
		if [ ! -d "$(TF_TEST_ROOT)" ]; then \
			echo "No IaC test root found: $(TF_TEST_ROOT)"; \
			exit 0; \
		fi; \
		test_dirs=$$(find "$(TF_TEST_ROOT)" -type f -path '*/tests/*.tftest.hcl' -print \
			| sed 's:/tests/[^/]*$$::' \
			| sort -u); \
	fi; \
	if [ -z "$$test_dirs" ]; then \
		echo "No .tftest.hcl files found under $(TF_TEST_ROOT)"; \
		exit 0; \
	fi; \
	for test_dir in $$test_dirs; do \
		if [ ! -d "$$test_dir" ]; then \
			echo "IaC test directory does not exist: $$test_dir"; \
			exit 1; \
		fi; \
		echo "make: Cleaning $$test_dir/.terraform and lock file before testing"; \
		rm -rf "$$test_dir/.terraform" "$$test_dir/.terraform.lock.hcl"; \
		echo "make: Running $(TF_TEST_CLI) acceptance test in $$test_dir"; \
		if [ -n "$$test_output" ]; then \
			run_logged "$(TF_TEST_RESOLVED_CLI_PATH)" -chdir="$$test_dir" init $(TF_TEST_INIT_ARGS); \
			run_logged "$(TF_TEST_RESOLVED_CLI_PATH)" -chdir="$$test_dir" test $(TF_TEST_ARGS); \
		else \
			"$(TF_TEST_RESOLVED_CLI_PATH)" -chdir="$$test_dir" init $(TF_TEST_INIT_ARGS); \
			"$(TF_TEST_RESOLVED_CLI_PATH)" -chdir="$$test_dir" test $(TF_TEST_ARGS); \
		fi; \
	done
	@if [ "$(WITH_SHARED_BUCKET)" = "1" ]; then \
		$(MAKE) --no-print-directory shared-bucket-test; \
	fi

tools: prereq-go ## Install tools
	@echo "make: Installing tools..."
	cd .ci/tools && $(GO_VER) install github.com/client9/misspell/cmd/misspell
	cd .ci/tools && $(GO_VER) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint
	cd .ci/tools && $(GO_VER) install github.com/katbyte/terrafmt
	cd .ci/tools && $(GO_VER) install github.com/pavius/impi/cmd/impi
	cd .ci/tools && $(GO_VER) install golang.org/x/tools/cmd/goimports
	cd .ci/tools && $(GO_VER) install golang.org/x/tools/cmd/stringer

# Please keep targets in alphabetical order
.PHONY: \
	build \
	ci \
	clean \
	clean-ci \
	clean-go \
	clean-make-tests \
	clean-terraform-examples \
	clean-tidy \
	default \
	deps-check \
	docs-lint \
	docs-misspell \
	fix-imports \
	fmt \
	fmt-check \
	gen \
	gen-check \
	gen-raw \
	go-build \
	go-misspell \
	golangci-lint \
	help \
	import-lint \
	lint \
	prereq-go \
	quick-fix \
	release \
	shared-bucket-create \
	shared-bucket-destroy \
	shared-bucket-test \
	sweep \
	sweeper-check \
	sweeper-linked \
	sweeper-unlinked \
	terraform-fmt \
	test \
	test-full \
	test-single-service \
	testacc \
	testacc-lint \
	testacc-lint-fix \
	testcompat \
	testiac \
	tools
