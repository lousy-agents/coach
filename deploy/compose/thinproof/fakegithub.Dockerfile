# Builds cmd/fakegithub-server: the internal-network Compose packaging of
# internal/fakegithub for issue #79's Task 0.3 thin offline proof. Tag
# coach/fakegithub-thinproof:0.1.0 (see docker-compose.yml).
# `mise run thinproof-build` runs `go mod vendor` (host-side, online) before
# this build, so the build stage below needs no network access at all --
# it builds with -mod=vendor against the vendor/ directory COPY . . picks up.
FROM golang:1.27-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -mod=vendor -o /out/fakegithub-server ./cmd/fakegithub-server

FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a
COPY --from=build /out/fakegithub-server /usr/local/bin/fakegithub-server
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/fakegithub-server"]
