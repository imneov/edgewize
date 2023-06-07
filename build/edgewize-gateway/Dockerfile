# Copyright 2020 The KubeSphere Authors. All rights reserved.
# Use of this source code is governed by an Apache license
# that can be found in the LICENSE file.

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

COPY --from=build_context /out/ /

WORKDIR /

EXPOSE 30000
EXPOSE 30002
EXPOSE 30004
CMD ["sh"]
