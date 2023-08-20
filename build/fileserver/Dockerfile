FROM alpine as builder

ADD https://github.com/edgewize-io/kubeedge/releases/download/v1.13.1/keadm-v1.13.1-linux-amd64.tar.gz /data/files/bin/v1.13.1/amd64/keadm-v1.13.1-linux-amd64.tar.gz
ADD https://github.com/edgewize-io/kubeedge/releases/download/v1.13.1/keadm-v1.13.1-linux-arm64.tar.gz /data/files/bin/v1.13.1/arm64/keadm-v1.13.1-linux-arm64.tar.gz
RUN chmod -R +r /data/

FROM edgewize/edge-ota-package:latest as ota
RUN chmod -R +r /data/

FROM nginx:alpine

COPY --from=ota /data/ /usr/share/nginx/html/edgeotafiles/
COPY --from=builder /data/files/ /usr/share/nginx/html/files/

EXPOSE 80