FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -trimpath -o /bridge .
FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 bridge && mkdir -p /var/lib/portaria-telephony && chown bridge:bridge /var/lib/portaria-telephony
COPY --from=build /bridge /usr/local/bin/bridge
USER bridge
ENTRYPOINT ["bridge"]
