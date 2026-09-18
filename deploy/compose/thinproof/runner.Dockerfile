# Builds cmd/thinproof-runner: the external test-runner container for issue
# #79's Task 0.3 thin offline proof. Tag coach/thinproof-runner:0.1.0 (see
# docker-compose.yml).
# `mise run thinproof-build` runs `go mod vendor` (host-side, online) before
# this build, so the build stage below needs no network access at all --
# it builds with -mod=vendor against the vendor/ directory COPY . . picks up.
FROM golang:1.27-alpine@sha256:4cb7ac979db5fcc41cae44b2227ba5ab8a51e8807f40d9ba4dee20a0ad960b5b AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -mod=vendor -o /out/thinproof-runner ./cmd/thinproof-runner

FROM alpine:3.24@sha256:e7c4abb69531cb09e2a2bbb56fad3367ab694865c49df898c1c683185cc4376c
COPY --from=build /out/thinproof-runner /usr/local/bin/thinproof-runner
ENTRYPOINT ["/usr/local/bin/thinproof-runner"]
