FROM --platform=$BUILDPLATFORM alpine:3.22.0 AS certs
RUN apk add --no-cache ca-certificates

FROM scratch
WORKDIR /

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY adguard-exporter /adguard-exporter
USER 65532:65532

ENTRYPOINT ["/adguard-exporter"]
