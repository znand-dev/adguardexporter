FROM golang:1.22 AS builder

WORKDIR /app
COPY . .

RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o adguard-exporter -ldflags="-s -w" main.go

# Final image
FROM scratch

COPY --from=builder /app/adguard-exporter /adguard-exporter

# For SSL root CAs
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

WORKDIR /
USER 65532:65532
ENTRYPOINT ["/adguard-exporter"]
