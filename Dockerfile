# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
ENV GOEXPERIMENT=jsonv2

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -o /out/wifire ./cmd

FROM alpine:3.21

RUN apk add --no-cache ca-certificates \
	&& adduser -D -u 1000 -g 1000 wifire

USER wifire:wifire

WORKDIR /app

COPY --from=build /out/wifire /usr/local/bin/wifire

ENTRYPOINT ["/usr/local/bin/wifire"]

# JSON grill log on the volume-mounted data directory (see docker-compose.yml).
CMD ["-o", "/data/grill.jsonl"]
