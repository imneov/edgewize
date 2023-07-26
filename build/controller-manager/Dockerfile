# Copyright 2020 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by an Apache license
# that can be found in the LICENSE file.

FROM golang:1.19.4 as build_context

ENV OUTDIR=/out
RUN mkdir -p ${OUTDIR}/usr/local/bin/
RUN mkdir -p ${OUTDIR}/var/helm-charts

WORKDIR /workspace
ADD . /workspace/

RUN make edgewize-controller-manager
RUN mv /workspace/bin/cmd/controller-manager ${OUTDIR}/usr/local/bin/

# Final Image

FROM edgewize/base:alpine-3.16.2

COPY --from=build_context /out/ /

COPY ./charts/edge /charts/edge

WORKDIR /

EXPOSE 8443 8080

CMD ["sh"]
