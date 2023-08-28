FROM alpine:3.16.2 as base_os_context

ARG TARGETARCH
ARG TARGETOS
ARG HELM_VERSION=v3.5.2
ARG KUBECTL_VERSION=v1.22.12

ENV OUTDIR=/out
RUN mkdir -p ${OUTDIR}/usr/local/bin/

WORKDIR /tmp

RUN apk add --no-cache ca-certificates

# install helm
ADD https://get.helm.sh/helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz /tmp
RUN tar xvzf /tmp/helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz -C /tmp
RUN mv /tmp/${TARGETOS}-${TARGETARCH}/helm ${OUTDIR}/usr/local/bin/
RUN chmod +x ${OUTDIR}/usr/local/bin/helm

# Install kubectl for development
ADD https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/${TARGETOS}/${TARGETARCH}/kubectl /tmp
RUN chmod +x /tmp/kubectl && mv /tmp/kubectl ${OUTDIR}/usr/local/bin/kubectl


FROM alpine:3.16.2

COPY --from=base_os_context /out/ /



