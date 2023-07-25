# Copyright 2020 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by an Apache license
# that can be found in the LICENSE file.

# Download dependencies
FROM alpine:3.16.2 as base_os_context

ARG TARGETARCH
ARG TARGETOS
ARG HELM_VERSION=v3.5.2
ARG KUSTOMIZE_VERSION=v4.2.0
ARG INGRESS_NGINX_VERSION=4.0.13

ENV OUTDIR=/out
RUN mkdir -p ${OUTDIR}/usr/local/bin
RUN mkdir -p ${OUTDIR}/var/helm-charts

WORKDIR /tmp

RUN apk add --no-cache ca-certificates

# Install helm
ADD https://get.helm.sh/helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz /tmp
RUN tar xvzf /tmp/helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz -C /tmp
RUN mv /tmp/${TARGETOS}-${TARGETARCH}/helm ${OUTDIR}/usr/local/bin/

# install kustomize
ADD https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_${TARGETOS}_${TARGETARCH}.tar.gz /tmp
RUN tar xvzf /tmp/kustomize_${KUSTOMIZE_VERSION}_${TARGETOS}_${TARGETARCH}.tar.gz -C /tmp
RUN mv /tmp/kustomize ${OUTDIR}/usr/local/bin/

# Install kubectl for development
ADD https://storage.googleapis.com/kubernetes-release/release/v1.22.12/bin/linux/amd64/kubectl /tmp
RUN chmod +x /tmp/kubectl && mv /tmp/kubectl ${OUTDIR}/usr/local/bin/kubectl

# Build

FROM golang:1.19.4 as build_context

ENV OUTDIR=/out
RUN mkdir -p ${OUTDIR}/usr/local/bin/
RUN mkdir -p ${OUTDIR}/var/helm-charts

WORKDIR /workspace
ADD . /workspace/

RUN make edgewize-controller-manager
RUN mv /workspace/bin/cmd/controller-manager ${OUTDIR}/usr/local/bin/

# Final Image

FROM alpine:3.16.2

COPY --from=base_os_context /out/ /
COPY --from=build_context /out/ /
COPY ./charts/edge /charts/edge

WORKDIR /

EXPOSE 8443 8080

CMD ["sh"]
