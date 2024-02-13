# syntax=docker/dockerfile:1

## Build
FROM golang:1.22-bookworm AS build

RUN apt update; apt install -y libvips-dev

ARG COMMIT_SHA=none
ARG COMMIT_BRANCH=none
ARG RELEASE=none

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN go build -ldflags="-X 'main.VersionCommit=$COMMIT_SHA' -X 'main.VersionBranch=$COMMIT_BRANCH' -X 'main.VersionRelease=$RELEASE'" -o /backend

## Deploy
FROM debian:bookworm

RUN apt update; apt install -y libvips42

WORKDIR /

COPY --from=build /backend /backend

ENTRYPOINT ["/backend"]
