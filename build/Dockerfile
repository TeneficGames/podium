# syntax=docker/dockerfile:1

# Podium leaderboard service
# https://github.com/TeneficGames/podium
# Licensed under the MIT license:
# http://www.opensource.org/licenses/mit-license
# Copyright © 2026 Tenefic Games

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

ENV CGO_ENABLED=0

WORKDIR /podium

COPY go.mod go.sum ./
COPY leaderboard/go.mod leaderboard/go.sum ./leaderboard/
COPY proto/go.mod proto/go.sum ./proto/

RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /podium/bin/podium ./main.go

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /podium/bin/podium /podium
COPY --from=build /podium/config/default.yaml /default.yaml

USER nonroot:nonroot

ENTRYPOINT ["/podium", "-c", "/default.yaml"]
