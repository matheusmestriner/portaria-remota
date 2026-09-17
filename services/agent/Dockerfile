FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -trimpath -o /agent .
FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -g 10001 portaria && adduser -D -u 10001 -G portaria portaria && mkdir -p /var/lib/portaria && chown portaria:portaria /var/lib/portaria
COPY --from=build /agent /usr/local/bin/agent
USER portaria
ENV AGENT_CONFIG=/etc/portaria/config.json
ENTRYPOINT ["agent"]
