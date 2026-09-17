FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod main.go ./
# Pin the upstream revision observed in the 2026-09-16 package documentation.
# Commit the generated go.mod/go.sum after integration validation in a Go environment.
RUN go get go.mau.fi/whatsmeow@2375e17 github.com/lib/pq@v1.10.9 && go mod tidy && CGO_ENABLED=0 go build -trimpath -o /bridge .
FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 bridge
COPY --from=build /bridge /usr/local/bin/bridge
USER bridge
EXPOSE 8090
ENTRYPOINT ["bridge"]
