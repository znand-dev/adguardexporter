# Stage 1: Builder
FROM golang:1.22 AS builder
WORKDIR /app

# Copy go.mod dan go.sum dulu supaya caching optimal
COPY go.mod go.sum ./

# Run go mod tidy atau download (ini sekarang aman)
RUN go mod download

# Sekarang copy semua source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o adguardexporter -ldflags="-s -w" main.go

# Stage 2: Final image
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/adguardexporter /usr/bin/adguardexporter

EXPOSE 9617
ENTRYPOINT ["/usr/bin/adguardexporter"]
