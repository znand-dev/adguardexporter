ARG BASE_IMAGE=alpine:3.21.0
FROM ${BASE_IMAGE} AS certs
RUN apk add --no-cache ca-certificates

FROM scratch
WORKDIR /

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY adguard-exporter /adguard-exporter
USER 65532:65532

ENTRYPOINT ["/adguard-exporter"]
