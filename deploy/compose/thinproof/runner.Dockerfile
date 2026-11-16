# Builds cmd/thinproof-runner: the external test-runner container for issue
# #79's Task 0.3 thin offline proof. Tag coach/thinproof-runner:0.1.0 (see
# docker-compose.yml).
# `mise run thinproof-build` runs `go mod vendor` (host-side, online) before
# this build, so the build stage below needs no network access at all --
# it builds with -mod=vendor against the vendor/ directory COPY . . picks up.
FROM golang:1.27-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -mod=vendor -o /out/thinproof-runner ./cmd/thinproof-runner

FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
COPY --from=build /out/thinproof-runner /usr/local/bin/thinproof-runner
ENTRYPOINT ["/usr/local/bin/thinproof-runner"]
