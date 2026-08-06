# Fitness Check-in MVP

This repository contains the Go workspace and shared contracts for the fitness check-in microservices.

## Requirements

- Go 1.23 or newer
- `protoc`, `protoc-gen-go`, and `protoc-gen-go-grpc` for generated gRPC code

## Commands

- `make proto` generates protobuf code when the required tools are installed.
- `go test ./pkg/...` runs shared package tests.
- `go vet ./pkg/...` checks shared packages.

Generated protobuf code belongs in each service's `internal/gen/` directory and must not be edited manually.
