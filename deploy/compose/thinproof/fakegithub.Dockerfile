# Builds cmd/fakegithub-server: the internal-network Compose packaging of
# internal/fakegithub for issue #79's Task 0.3 thin offline proof. Tag
# coach/fakegithub-thinproof:0.1.0 (see docker-compose.yml).
# `mise run thinproof-build` runs `go mod vendor` (host-side, online) before
# this build, so the build stage below needs no network access at all --
# it builds with -mod=vendor against the vendor/ directory COPY . . picks up.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -mod=vendor -o /out/fakegithub-server ./cmd/fakegithub-server

FROM alpine:3.20
COPY --from=build /out/fakegithub-server /usr/local/bin/fakegithub-server
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/fakegithub-server"]
