# Builds cmd/thinproof-runner: the external test-runner container for issue
# #79's Task 0.3 thin offline proof. Tag coach/thinproof-runner:0.1.0 (see
# docker-compose.yml).
# `mise run thinproof-build` runs `go mod vendor` (host-side, online) before
# this build, so the build stage below needs no network access at all --
# it builds with -mod=vendor against the vendor/ directory COPY . . picks up.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -mod=vendor -o /out/thinproof-runner ./cmd/thinproof-runner

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
COPY --from=build /out/thinproof-runner /usr/local/bin/thinproof-runner
ENTRYPOINT ["/usr/local/bin/thinproof-runner"]
