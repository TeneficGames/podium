# podium - webhook dispatching service
# https://github.com/topfreegames/podium
# Licensed under the MIT license:
# http://www.opensource.org/licenses/mit-license
# Copyright © 2019 Top Free Games <backend@tfgco.com>

FROM golang:1.26-alpine as build

MAINTAINER TFG Co <backend@tfgco.com>

ENV CGO_ENABLED=0

COPY . /podium

WORKDIR /podium

RUN apk update && apk add make && make setup
RUN go mod tidy && make build

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /podium/bin/podium /podium
COPY --from=build /podium/config/default.yaml /default.yaml

ENTRYPOINT ["/podium", "-c", "/default.yaml"]
