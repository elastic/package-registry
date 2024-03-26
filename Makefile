PLATFORMS ?= linux/amd64 linux/arm64/v8 linux/arm64
PLATFORM_TARGETS=$(addprefix release-, $(PLATFORMS))

TARGET_ARCH_amd64=x86_64
TARGET_ARCH_arm64=arm64

OSS ?= linux
OSS_TARGETS=$(addprefix release-, $(OSS))

.PHONY: $(OSS_TARGETS)
$(OSS_TARGETS): release-%:
	$(eval $@_OS := $(firstword $(subst /, ,$(lastword $(subst release-, ,$@)))))
	GOOS=$($@_OS) go build .

.PHONY: $(OS_TARGETS)
$(PLATFORM_TARGETS): release-%:
	$(eval $@_OS := $(firstword $(subst /, ,$(lastword $(subst release-, ,$@)))))
	$(eval $@_GO_ARCH := $(word 2, $(subst /, ,$(lastword $(subst release-, ,$@)))))
	$(eval $@_ARCH := $(TARGET_ARCH_$($@_GO_ARCH)))
	GOOS=$($@_OS) GOARCH=$($@_GO_ARCH) go build .
