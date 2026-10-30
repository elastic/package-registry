PLATFORMS ?= linux/amd64 linux/arm64/v8 linux/arm64
PLATFORM_TARGETS=$(addprefix release-, $(PLATFORMS))

TARGET_ARCH_amd64=x86_64
TARGET_ARCH_arm64=arm64

OSS ?= linux
OSS_TARGETS=$(addprefix release-, $(OSS))

BUILD_FLAGS=-tags=grpcnotrace -trimpath -ldflags=-s

# FIPS=1 pins the build to the certified Go FIPS 140-3 crypto module
# (see https://go.dev/doc/security/fips140#fips-140-3-mode).
FIPS ?= 0
ifeq ($(FIPS),1)
GO_BUILD_ENV=GOFIPS140=v1.0.0
else
GO_BUILD_ENV=
endif

# BATS versions
# https://bats-core.readthedocs.io/en/stable/writing-tests.html#special-variables
BATS_CORE_VERSION=v1.14.0
BATS_ASSERT_VERSION=v2.2.4
BATS_SUPPORT_VERSION=v0.3.0
BATS_FILE_VERSION=v0.4.0
BATS_SOURCE_DIR=$(CURDIR)/bats_tests/test_helper
BATS_WORK_DIR_TESTS=$(CURDIR)/tmp_bats_exec

.PHONY: $(OSS_TARGETS)
$(OSS_TARGETS): release-%:
	$(eval $@_OS := $(firstword $(subst /, ,$(lastword $(subst release-, ,$@)))))
	$(GO_BUILD_ENV) GOOS=$($@_OS) go build $(BUILD_FLAGS) .

.PHONY: $(OS_TARGETS)
$(PLATFORM_TARGETS): release-%:
	$(eval $@_OS := $(firstword $(subst /, ,$(lastword $(subst release-, ,$@)))))
	$(eval $@_GO_ARCH := $(word 2, $(subst /, ,$(lastword $(subst release-, ,$@)))))
	$(eval $@_ARCH := $(TARGET_ARCH_$($@_GO_ARCH)))
	$(GO_BUILD_ENV) GOOS=$($@_OS) GOARCH=$($@_GO_ARCH) go build $(BUILD_FLAGS) .

# Clone a bats repository at the given tag, replacing any existing checkout of another version.
# Usage: $(call bats_install,<repository>,<tag>)
define bats_install
	@echo "Prepare $(1) repo"
	@[ "$$(git -C $(BATS_SOURCE_DIR)/$(1) describe --tags --exact-match 2>/dev/null)" = "$(2)" ] \
	  || { rm -rf $(BATS_SOURCE_DIR)/$(1) \
	       && git -c advice.detachedHead=false clone --quiet -b $(2) --depth 1 https://github.com/bats-core/$(1) $(BATS_SOURCE_DIR)/$(1); }
endef

.PHONY: bats bats-clean bats-tester bats-tester-ci
bats: ## Install bats and libraries in the project itself
	$(call bats_install,bats-core,$(BATS_CORE_VERSION))
	$(call bats_install,bats-assert,$(BATS_ASSERT_VERSION))
	$(call bats_install,bats-support,$(BATS_SUPPORT_VERSION))
	$(call bats_install,bats-file,$(BATS_FILE_VERSION))

bats-clean:
	@echo "Cleaning bats directories"
	rm -rf $(BATS_SOURCE_DIR)
	rm -rf $(BATS_WORK_DIR_TESTS)

bats-tester-ci: bats
	@echo "Running tests bats"
	@rm -rf $(BATS_WORK_DIR_TESTS)
	@mkdir -p $(BATS_WORK_DIR_TESTS)
	@cd $(BATS_WORK_DIR_TESTS) && \
	  $(BATS_SOURCE_DIR)/bats-core/bin/bats --formatter pretty --report-formatter junit --output $(BATS_WORK_DIR_TESTS) $(CURDIR)/.buildkite/scripts/tests; \
	  status=$$?; \
	  if [ -f report.xml ]; then mv report.xml $(CURDIR)/tests-report-bats.xml; fi; \
	  cd $(CURDIR) && rm -rf $(BATS_WORK_DIR_TESTS); \
	  exit $$status

bats-tester: bats
	@echo "Running tests bats"
	@rm -rf "$(BATS_WORK_DIR_TESTS)"
	@mkdir -p $(BATS_WORK_DIR_TESTS)
	@cd $(BATS_WORK_DIR_TESTS) && \
	  $(BATS_SOURCE_DIR)/bats-core/bin/bats --verbose-run --formatter pretty $(CURDIR)/.buildkite/scripts/tests
	@rm -rf "$(BATS_WORK_DIR_TESTS)"
