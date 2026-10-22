# Builds cmd/thinproof-runner: the external test-runner container for issue
# #79's Task 0.3 thin offline proof. Tag coach/thinproof-runner:0.1.0 (see
# docker-compose.yml).
# `mise run thinproof-build` runs `go mod vendor` (host-side, online) before
# this build, so the build stage below needs no network access at all --
# it builds with -mod=vendor against the vendor/ directory COPY . . picks up.
FROM golang:1.27-alpine@sha256:b8c9b82affbbb82a020bbcf998dad2ba9516944ac7e883524642a481a9505e9c AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -mod=vendor -o /out/thinproof-runner ./cmd/thinproof-runner

FROM alpine:3.24@sha256:71e9b7bae160ba167fa17414401b94cbcdaf3c32feeab9298bd794a2a0614c35
COPY --from=build /out/thinproof-runner /usr/local/bin/thinproof-runner
ENTRYPOINT ["/usr/local/bin/thinproof-runner"]
