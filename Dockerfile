FROM golang:1.22 AS builder

WORKDIR /app
COPY . ./

RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o adguard-exporter -ldflags="-s -w" main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /app/adguard-exporter .

EXPOSE 9200
ENTRYPOINT ["./adguard-exporter"]
