#!/usr/bin/env bash

set -ex
set -o pipefail

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${KUBE_ROOT}/hack/lib/init.sh"

# push to kubesphere with default latest tag
TAG=${TAG:-latest}
REPO=${REPO:-kubesphere}

# If set, just building, no pushing
DRY_RUN=${DRY_RUN:-}

# support other container tools. e.g. podman
CONTAINER_CLI=${CONTAINER_CLI:-docker}
CONTAINER_BUILDER=${CONTAINER_BUILDER:-build}

# use host os and arch as default target os and arch
TARGETOS=${TARGETOS:-$(kube::util::host_os)}
TARGETARCH=${TARGETARCH:-$(kube::util::host_arch)}

${CONTAINER_CLI} "${CONTAINER_BUILDER}" \
  --build-arg TARGETARCH="${TARGETARCH}" \
  --build-arg TARGETOS="${TARGETOS}" \
  -f build/apiserver/Dockerfile \
  -t "${REPO}"/edgewize-apiserver:"${TAG}" .


${CONTAINER_CLI} "${CONTAINER_BUILDER}" \
  --build-arg "TARGETARCH=${TARGETARCH}" \
  --build-arg "TARGETOS=${TARGETOS}" \
  -f build/controller-manager/Dockerfile \
  -t "${REPO}"/edgewize-controller-manager:"${TAG}" .

${CONTAINER_CLI} "${CONTAINER_BUILDER}" \
  --build-arg "TARGETARCH=${TARGETARCH}" \
  --build-arg "TARGETOS=${TARGETOS}" \
  -f build/gateway/Dockerfile \
  -t "${REPO}"/edgewize-gateway:"${TAG}" .

if [[ -z "${DRY_RUN:-}" ]]; then
  ${CONTAINER_CLI} push "${REPO}"/edgewize-apiserver:"${TAG}"
  ${CONTAINER_CLI} push "${REPO}"/edgewize-controller-manager:"${TAG}"
  ${CONTAINER_CLI} push "${REPO}"/edgewize-gateway:"${TAG}"
fi
