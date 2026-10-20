# Builds cmd/fakegithub-server: the internal-network Compose packaging of
# internal/fakegithub for issue #79's Task 0.3 thin offline proof. Tag
# coach/fakegithub-thinproof:0.1.0 (see docker-compose.yml).
# `mise run thinproof-build` runs `go mod vendor` (host-side, online) before
# this build, so the build stage below needs no network access at all --
# it builds with -mod=vendor against the vendor/ directory COPY . . picks up.
FROM golang:1.27-alpine@sha256:f2c872bcee949b7be9c490efd02b48a8a2fbf947cdf533f1c58525ec9d8170fd AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -mod=vendor -o /out/fakegithub-server ./cmd/fakegithub-server

FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
COPY --from=build /out/fakegithub-server /usr/local/bin/fakegithub-server
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/fakegithub-server"]
