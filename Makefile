# Copyright 2018 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by a Apache license
# that can be found in the LICENSE file.


# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:allowDangerousTypes=true"

GV="infra:v1alpha1 apps:v1alpha1 alerting:v2beta1"
MANIFESTS="infra/* apps/* alerting/v2beta1"

# App Version
APP_VERSION = v0.1.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

OUTPUT_DIR=bin
ifeq (${GOFLAGS},)
	# go build with vendor by default.
	export GOFLAGS=-mod=vendor
endif
define ALL_HELP_INFO
# Build code.
#
# Args:
#   WHAT: Directory names to build.  If any of these directories has a 'main'
#     package, the build will produce executable files under $(OUT_DIR).
#     If not specified, "everything" will be built.
#   GOFLAGS: Extra flags to pass to 'go' when building.
#   GOLDFLAGS: Extra linking flags passed to 'go' when building.
#   GOGCFLAGS: Additional go compile flags passed to 'go' when building.
#
# Example:
#   make
#   make all
#   make all WHAT=cmd/edgewize-apiserver
#     Note: Use the -N -l options to disable compiler optimizations an inlining.
#           Using these build options allows you to subsequently use source
#           debugging tools like delve.
endef
.PHONY: all
all: test edgewize-apiserver edgewize-controller-manager;$(info $(M)...Begin to test and build all of binary.) @ ## Test and build all of binary.

help:
	@grep -hE '^[ a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-17s\033[0m %s\n", $$1, $$2}'

.PHONY: binary
# Build all of binary
binary: | edgewize-apiserver edgewize-controller-manager ; $(info $(M)...Build all of binary.) @ ## Build all of binary.

# Build edgewize-apiserver binary
edgewize-apiserver: ; $(info $(M)...Begin to build edgewize-apiserver binary.)  @ ## Build edgewize-apiserver.
	hack/gobuild.sh cmd/edgewize-apiserver;

# Build edgewize-controller-manager binary
edgewize-controller-manager: ; $(info $(M)...Begin to build edgewize-controller-manager binary.)  @ ## Build ks-controller-manager.
	hack/gobuild.sh cmd/controller-manager

# Run all verify scripts hack/verify-*.sh
verify-all: ; $(info $(M)...Begin to run all verify scripts hack/verify-*.sh.)  @ ## Run all verify scripts hack/verify-*.sh.
	hack/verify-all.sh

# Build e2e binary
e2e: ;$(info $(M)...Begin to build e2e binary.)  @ ## Build e2e binary.
	hack/build_e2e.sh test/e2e

kind-e2e: ;$(info $(M)...Run e2e test.) @ ## Run e2e test in kind.
	hack/kind_e2e.sh

# Run go fmt against code
fmt: ;$(info $(M)...Begin to run go fmt against code.)  @ ## Run go fmt against code.
	gofmt -w ./pkg ./cmd ./tools ./api

# Format all import, `goimports` is required.
goimports: ;$(info $(M)...Begin to Format all import.)  @ ## Format all import, `goimports` is required.
	@hack/update-goimports.sh

# Run go vet against code
vet: ;$(info $(M)...Begin to run go vet against code.)  @ ## Run go vet against code.
	go vet ./pkg/... ./cmd/...

# Generate manifests e.g. CRD, RBAC etc.
.PHONY: manifests
manifests: ;$(info $(M)...Begin to generate manifests e.g. CRD, RBAC etc..)  @ ## Generate manifests e.g. CRD, RBAC etc.
	hack/generate_manifests.sh ${CRD_OPTIONS} ${MANIFESTS}

deploy: manifests ;$(info $(M)...Begin to deploy.)  @ ## Deploy.
	kubectl apply -f config/crds
	kustomize build config/default | kubectl apply -f -

mockgen: ;$(info $(M)...Begin to mockgen.)  @ ## Mockgen.
	mockgen -package=openpitrix -source=pkg/simple/client/openpitrix/openpitrix.go -destination=pkg/simple/client/openpitrix/mock.go

deepcopy: ;$(info $(M)...Begin to deepcopy.)  @ ## Deepcopy.
	hack/generate_group.sh "deepcopy" kubesphere.io/api kubesphere.io/api ${GV} --output-base=staging/src/  -h "hack/boilerplate.go.txt"

openapi: ;$(info $(M)...Begin to openapi.)  @ ## Openapi.
	go run ./vendor/k8s.io/kube-openapi/cmd/openapi-gen/openapi-gen.go -O openapi_generated -i ./vendor/k8s.io/apimachinery/pkg/apis/meta/v1,./vendor/kubesphere.io/api/tenant/v1alpha1 -p kubesphere.io/api/tenant/v1alpha1 -h ./hack/boilerplate.go.txt --report-filename ./api/api-rules/violation_exceptions.list  --output-base=staging/src/
	go run ./vendor/k8s.io/kube-openapi/cmd/openapi-gen/openapi-gen.go -O openapi_generated -i ./vendor/k8s.io/apimachinery/pkg/apis/meta/v1,./vendor/kubesphere.io/api/network/v1alpha1 -p kubesphere.io/api/network/v1alpha1 -h ./hack/boilerplate.go.txt --report-filename ./api/api-rules/violation_exceptions.list  --output-base=staging/src/
	go run ./vendor/k8s.io/kube-openapi/cmd/openapi-gen/openapi-gen.go -O openapi_generated -i ./vendor/k8s.io/apimachinery/pkg/apis/meta/v1,./vendor/kubesphere.io/api/servicemesh/v1alpha2 -p kubesphere.io/api/servicemesh/v1alpha2 -h ./hack/boilerplate.go.txt --report-filename ./api/api-rules/violation_exceptions.list  --output-base=staging/src/
	go run ./vendor/k8s.io/kube-openapi/cmd/openapi-gen/openapi-gen.go -O openapi_generated -i ./vendor/k8s.io/api/networking/v1,./vendor/k8s.io/apimachinery/pkg/apis/meta/v1,./vendor/k8s.io/apimachinery/pkg/util/intstr,./vendor/kubesphere.io/api/network/v1alpha1 -p kubesphere.io/api/network/v1alpha1 -h ./hack/boilerplate.go.txt --report-filename ./api/api-rules/violation_exceptions.list  --output-base=staging/src/
	go run ./vendor/k8s.io/kube-openapi/cmd/openapi-gen/openapi-gen.go -O openapi_generated -i ./vendor/k8s.io/apimachinery/pkg/apis/meta/v1,./vendor/kubesphere.io/api/devops/v1alpha1,./vendor/k8s.io/apimachinery/pkg/runtime,./vendor/k8s.io/api/core/v1 -p kubesphere.io/api/devops/v1alpha1 -h ./hack/boilerplate.go.txt --report-filename ./api/api-rules/violation_exceptions.list  --output-base=staging/src/
	go run ./vendor/k8s.io/kube-openapi/cmd/openapi-gen/openapi-gen.go -O openapi_generated -i ./vendor/k8s.io/apimachinery/pkg/apis/meta/v1,./vendor/kubesphere.io/api/cluster/v1alpha1,./vendor/k8s.io/apimachinery/pkg/runtime,./vendor/k8s.io/api/core/v1 -p kubesphere.io/api/cluster/v1alpha1 -h ./hack/boilerplate.go.txt --report-filename ./api/api-rules/violation_exceptions.list  --output-base=staging/src/
	go run ./vendor/k8s.io/kube-openapi/cmd/openapi-gen/openapi-gen.go -O openapi_generated -i ./vendor/k8s.io/apimachinery/pkg/apis/meta/v1,./vendor/kubesphere.io/api/devops/v1alpha3,./vendor/k8s.io/apimachinery/pkg/runtime -p kubesphere.io/api/devops/v1alpha3 -h ./hack/boilerplate.go.txt --report-filename ./api/api-rules/violation_exceptions.list  --output-base=staging/src/
	go run ./tools/cmd/crd-doc-gen/main.go
	go run ./tools/cmd/doc-gen/main.go

base-image: ;$(info $(M)...Begin to build the base docker image.)  @ ## Build the docker image.
	DRY_RUN=true hack/docker_build_base.sh

base-image-push: ;$(info $(M)...Begin to build the base docker image.)  @ ## Build the docker image.
	hack/docker_build_base.sh

container: ;$(info $(M)...Begin to build the docker image.)  @ ## Build the docker image.
	DRY_RUN=true hack/docker_build.sh

container-push: ;$(info $(M)...Begin to build and push.)  @ ## Build and Push.
	hack/docker_build.sh

container-cross: ; $(info $(M)...Begin to build container images for multiple platforms.)  @ ## Build container images for multiple platforms. Currently, only linux/amd64,linux/arm64 are supported.
	DRY_RUN=true hack/docker_build_multiarch.sh

container-cross-push: ; $(info $(M)...Begin to build and push.)  @ ## Build and Push.
	hack/docker_build_multiarch.sh

fileserver: ;$(info $(M)...Begin to build the docker image.)  @ ## Build the docker image.
	DRY_RUN=true hack/fileserver_build.sh

fileserver-push: ;$(info $(M)...Begin to build and push.)  @ ## Build and Push.
	hack/fileserver_build.sh

helm-package: ; $(info $(M)...Begin to helm-package.)  @ ## Helm-package.
	ls config/crds/*.edgewize.io* | xargs -i cp -r {} charts/host/edgewize/crds/
	ls config/crds/*.edgewize.io* | xargs -i cp -r {} charts/edge/edgewize/crds/
	helm package charts/host/edgewize --app-version=${APP_VERSION} --version=${APP_VERSION} -d ./bin

helm-deploy: ; $(info $(M)...Begin to helm-deploy.)  @ ## Helm-deploy.
	ls config/crds/ | xargs -i cp -r config/crds/{} charts/host/edgewize/crds/
	- kubectl create ns edgewize-system
	helm upgrade --install edgewize ./charts/host/edgewize -n edgewize-system --create-namespace

helm-uninstall: ; $(info $(M)...Begin to helm-uninstall.)  @ ## Helm-uninstall.
	- kubectl delete ns edgewize-system
	helm uninstall edgewize -n edgewize-system

# Run tests
ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
test: vet test-env ;$(info $(M)...Begin to run tests.)  @ ## Run tests.
	export KUBEBUILDER_ASSETS=$(shell pwd)/testbin/bin; go test ./pkg/... ./cmd/... -covermode=atomic -coverprofile=coverage.txt
	cd staging/src/kubesphere.io/api ; GOFLAGS="" go test ./...
	cd staging/src/kubesphere.io/client-go ; GOFLAGS="" go test ./...

.PHONY: test-env
test-env: ;$(info $(M)...Begin to setup test env) @ ## Download unit test libraries e.g. kube-apiserver etcd.
	@hack/setup-kubebuilder-env.sh

.PHONY: clean
clean: ;$(info $(M)...Begin to clean.)  @ ## Clean.
	-make -C ./pkg/version clean
	@echo "ok"

clientset:  ;$(info $(M)...Begin to find or download controller-gen.)  @ ## Find or download controller-gen,download controller-gen if necessary.
	./hack/generate_client.sh ${GV}

# Fix invalid file's license.
update-licenses: ;$(info $(M)...Begin to update licenses.)
	@hack/update-licenses.sh
