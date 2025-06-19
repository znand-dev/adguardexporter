FROM golang:1.24 AS builder
WORKDIR /app

COPY . .

RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o adguardexporter -ldflags="-s -w" main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/adguardexporter /usr/bin/adguardexporter

EXPOSE 9617
ENTRYPOINT ["/usr/bin/adguardexporter"]
