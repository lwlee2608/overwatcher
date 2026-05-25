FROM golang:1.25 AS build-env

ARG APP=overwatcher
ARG VERSION=dev
ARG COMMIT_SHA=dev

ENV GO111MODULE=on  \
    CGO_ENABLED=0   \
    GOOS=linux      \
    GOARCH=amd64

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/${APP} ${APP}/
COPY internal/ internal/
RUN go build -o app -ldflags "-X main.AppVersion=${VERSION}-${COMMIT_SHA}" ${APP}/*.go


FROM alpine

ARG APP

WORKDIR /go/bin
RUN apk add --no-cache ca-certificates
COPY application.yml /go/bin/application.yml
COPY --from=build-env /build/app .
RUN chmod +x app

ENTRYPOINT ["/go/bin/app"]
