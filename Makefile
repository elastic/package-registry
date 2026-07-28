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
