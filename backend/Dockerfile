# One image, four binaries: api (default), migrate, seed, admin.
# Base images are pinned by digest; Dependabot (docker ecosystem) proposes updates.
# Go must stay in sync with .github/workflows/ci.yml (GO_VERSION).
FROM golang:1.27-alpine3.24@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download && go mod verify

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate && \
    go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed && \
    go build -trimpath -ldflags="-s -w" -o /out/admin ./cmd/admin

FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
# ca-certificates: outgoing TLS; tzdata: Asia/Almaty etc.; busybox wget is used by the healthcheck.
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S -g 10001 app && adduser -S -D -H -u 10001 -G app app
COPY --from=build /out/ /usr/local/bin/
USER 10001:10001
ENV HTTP_ADDR=:8080
EXPOSE 8080
CMD ["api"]
