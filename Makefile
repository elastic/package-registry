PLATFORMS ?= linux/amd64 linux/arm64/v8 linux/arm64
PLATFORM_TARGETS=$(addprefix release-, $(PLATFORMS))
PLATFORM_FIPS_TARGETS=$(addprefix release-fips-, $(PLATFORMS))

TARGET_ARCH_amd64=x86_64
TARGET_ARCH_arm64=arm64

OSS ?= linux
OSS_TARGETS=$(addprefix release-, $(OSS))

BUILD_FLAGS=-tags=grpcnotrace -trimpath -ldflags=-s
FIPS_BUILD_FLAGS=-tags=grpcnotrace -trimpath -ldflags=-s

.PHONY: $(OSS_TARGETS)
$(OSS_TARGETS): release-%:
	$(eval $@_OS := $(firstword $(subst /, ,$(lastword $(subst release-, ,$@)))))
	GOOS=$($@_OS) go build $(BUILD_FLAGS) .

.PHONY: $(OS_TARGETS)
$(PLATFORM_TARGETS): release-%:
	$(eval $@_OS := $(firstword $(subst /, ,$(lastword $(subst release-, ,$@)))))
	$(eval $@_GO_ARCH := $(word 2, $(subst /, ,$(lastword $(subst release-, ,$@)))))
	$(eval $@_ARCH := $(TARGET_ARCH_$($@_GO_ARCH)))
	GOOS=$($@_OS) GOARCH=$($@_GO_ARCH) go build $(BUILD_FLAGS) .

.PHONY: $(PLATFORM_FIPS_TARGETS)
$(PLATFORM_FIPS_TARGETS): release-fips-%:
	$(eval $@_OS := $(firstword $(subst /, ,$(lastword $(subst release-fips-, ,$@)))))
	$(eval $@_GO_ARCH := $(word 2, $(subst /, ,$(lastword $(subst release-fips-, ,$@)))))
	$(eval $@_ARCH := $(TARGET_ARCH_$($@_GO_ARCH)))
	@echo ">> Downloading dependencies for FIPS build"
	@go mod download
	@echo ">> Building with FIPS 140 enabled"
	GOFIPS140=v1.0.0 GODEBUG=fips140=only GOOS=$($@_OS) GOARCH=$($@_GO_ARCH) go build $(FIPS_BUILD_FLAGS) .
