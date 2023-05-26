# Copyright 2020 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by an Apache license
# that can be found in the LICENSE file.

# Download dependencies
FROM alpine:3.16.2 as base_os_context

ARG TARGETARCH
ARG TARGETOS

ENV OUTDIR=/out
RUN mkdir -p ${OUTDIR}/usr/local/bin/

WORKDIR /tmp

RUN apk add --no-cache ca-certificates

# Build
FROM golang:1.19.4 as build_context

ENV OUTDIR=/out
RUN mkdir -p ${OUTDIR}/usr/local/bin/

WORKDIR /workspace
ADD . /workspace/

RUN make edgewize-gateway
RUN mv /workspace/bin/cmd/edgewize-gateway ${OUTDIR}/usr/local/bin/

##############
# Final image
#############

FROM alpine:3.16.2

COPY --from=base_os_context /out/ /
COPY --from=build_context /out/ /

WORKDIR /

EXPOSE 9090
CMD ["sh"]
