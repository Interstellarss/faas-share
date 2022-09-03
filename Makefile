TARGET=kubeshare-scheduler kubeshare-device-manager kubeshare-config-client
GO=go
GO_MODULE=GO111MODULE=on
BIN_DIR=bin/
ALPINE_COMPILE_FLAGS=CGO_ENABLED=0 GOOS=linux GOARCH=amd64
NVML_COMPILE_FLAGS=CGO_LDFLAGS_ALLOW='-Wl,--unresolved-symbols=ignore-in-object-files' GOOS=linux GOARCH=amd64
PACKAGE_PREFIX=github.com/Interstellarss/faas-share/cmd/

.PHONY: build local push namespaces install charts start-kind stop-kind build-buildx render-charts all clean $(TARGET)
IMG_NAME?=faas-share

TAG?=v0.1.23
OWNER?=interstellarss
SERVER?=ghcr.io
export DOCKER_CLI_EXPERIMENTAL=enabled
export DOCKER_BUILDKIT=1



TOOLS_DIR := .tools

GOPATH := $(shell go env GOPATH)
CODEGEN_VERSION := $(shell hack/print-codegen-version.sh)
CODEGEN_PKG := $(GOPATH)/pkg/mod/k8s.io/code-generator@${CODEGEN_VERSION}

VERSION := $(shell git describe --tags --dirty)
GIT_COMMIT := $(shell git rev-parse HEAD)

all: build-docker $(TARGET)

$(TOOLS_DIR)/code-generator.mod: go.mod
	@echo "syncing code-generator tooling version"
	@cd $(TOOLS_DIR) && go mod edit -require "k8s.io/code-generator@${CODEGEN_VERSION}"

${CODEGEN_PKG}: $(TOOLS_DIR)/code-generator.mod
	@echo "(re)installing k8s.io/code-generator-${CODEGEN_VERSION}"
	@cd $(TOOLS_DIR) && go mod download -modfile=code-generator.mod

local:
	CGO_ENABLED=0 go build -a -installsuffix cgo -o faas-share

build-docker:
	docker build \
	--build-arg GIT_COMMIT=$(GIT_COMMIT) \
	--build-arg VERSION=$(VERSION) \
	-t $(SERVER)/$(OWNER)/$(IMG_NAME):$(TAG) .

.PHONY: build-buildx
build-buildx:
	@echo $(SERVER)/$(OWNER)/$(IMG_NAME):$(TAG) && \
	docker buildx create --use --name=multiarch --node=multiarch && \
	docker buildx build \
		--push \
		--platform linux/amd64 \
        --build-arg GIT_COMMIT=$(GIT_COMMIT) \
        --build-arg VERSION=$(VERSION) \
		--tag $(SERVER)/$(OWNER)/$(IMG_NAME):$(TAG) \
		.

.PHONY: build-buildx-all
build-buildx-all:
	@docker buildx create --use --name=multiarch --node=multiarch && \
	docker buildx build \
		--platform linux/amd64,linux/arm/v7,linux/arm64 \
		--output "type=image,push=false" \
        --build-arg GIT_COMMIT=$(GIT_COMMIT) \
        --build-arg VERSION=$(VERSION) \
		--tag $(SERVER)/$(OWNER)/$(IMG_NAME):$(TAG) \
		.

.PHONY: publish-buildx-all
publish-buildx-all:
	@echo  $(SERVER)/$(OWNER)/$(IMG_NAME):$(TAG) && \
	docker buildx create --use --name=multiarch --node=multiarch && \
	docker buildx build \
		--platform linux/amd64,linux/arm/v7,linux/arm64 \
		--push=true \
        --build-arg GIT_COMMIT=$(GIT_COMMIT) \
        --build-arg VERSION=$(VERSION) \
		--tag $(SERVER)/$(OWNER)/$(IMG_NAME):$(TAG) \
		.

push:
	docker push $(SERVER)/$(OWNER)/$(IMG_NAME):$(TAG)

charts: ## helm repo index docs --url https://interstellarss.github.io/faas-share --merge ./docs/index.yaml
	cd chart && helm package faas-share/ 
	mv chart/*.tgz docs/
	helm repo index docs --url https://interstellarss.github.io/faas-share-charts --merge ./docs/index.yaml
	./contrib/create-static-manifest.sh

render-charts:
	./contrib/create-static-manifest.sh
	./contrib/create-static-manifest.sh ./chart/faas-share ./yaml_arm64 ./chart/faas-share/values-arm64.yaml
	./contrib/create-static-manifest.sh ./chart/faas-share ./yaml_armhf ./chart/faas-share/values-armhf.yaml

start-kind: ## attempt to start a new dev environment
	@./contrib/create_dev.sh

stop-kind: ## attempt to stop the dev environment
	@./contrib/stop_dev.sh

.PHONY: verify-codegen
verify-codegen: ${CODEGEN_PKG}
	./hack/verify-codegen.sh

.PHONY: update-codegen
update-codegen: ${CODEGEN_PKG}
	./hack/update-codegen.sh

kubeshare-device-manager kubeshare-scheduler:
	$(GO_MODULE) $(ALPINE_COMPILE_FLAGS) $(GO) build -o $(BIN_DIR)$@ $(PACKAGE_PREFIX)$@

kubeshare-config-client:
	$(GO_MODULE) $(NVML_COMPILE_FLAGS) $(GO) build -o $(BIN_DIR)$@ $(PACKAGE_PREFIX)$@

clean:
	rm $(BIN_DIR)* 2>/dev/null; exit 0