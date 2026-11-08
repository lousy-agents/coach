# Builds cmd/fakegithub-server: the internal-network Compose packaging of
# internal/fakegithub for issue #79's Task 0.3 thin offline proof. Tag
# coach/fakegithub-thinproof:0.1.0 (see docker-compose.yml).
# `mise run thinproof-build` runs `go mod vendor` (host-side, online) before
# this build, so the build stage below needs no network access at all --
# it builds with -mod=vendor against the vendor/ directory COPY . . picks up.
FROM golang:1.27-alpine@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -mod=vendor -o /out/fakegithub-server ./cmd/fakegithub-server

FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
COPY --from=build /out/fakegithub-server /usr/local/bin/fakegithub-server
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/fakegithub-server"]
