FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY adguard-exporter .

EXPOSE 9200
ENTRYPOINT ["./adguard-exporter"]
