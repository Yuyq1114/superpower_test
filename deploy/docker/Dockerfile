# syntax=docker/dockerfile:1.7
FROM golang:1.25-alpine AS build
ARG SERVICE
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY pkg ./pkg
COPY proto ./proto
COPY services ./services
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build test -n "$SERVICE" && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/service "./services/${SERVICE}/cmd"

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build --chown=nonroot:nonroot /out/service /app/service
USER nonroot:nonroot
ENTRYPOINT ["/app/service"]
