# syntax = docker/dockerfile:experimental
FROM golang:1.19.11 AS build
WORKDIR /go/src/app
COPY . .
ENV GOPROXY=https://goproxy.io,direct
ENV GOPATH=/go
RUN --mount=type=cache,id=golang,target=/go/pkg/mod go install .

FROM ubuntu:22.04
WORKDIR /opt/
COPY --from=build /go/bin/canal_exporter .
CMD ["/opt/canal_exporter"]
