FROM golang:1.25 AS build-env

ARG APP=overwatcher
ARG COMMIT_SHA=
ARG RAILWAY_GIT_COMMIT_SHA

ENV GO111MODULE=on  \
    CGO_ENABLED=0   \
    GOOS=linux      \
    GOARCH=amd64

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/${APP} ${APP}/
COPY internal/ internal/
COPY VERSION ./
RUN VERSION=$(cat VERSION) && \
    SHA=$(echo "${RAILWAY_GIT_COMMIT_SHA:-$COMMIT_SHA}" | cut -c1-7) && \
    go build -o app -ldflags "-X main.AppVersion=${VERSION}${SHA:+-$SHA}" ${APP}/*.go


FROM alpine

ARG APP

WORKDIR /go/bin
RUN apk add --no-cache ca-certificates
COPY application.yml /go/bin/application.yml
COPY --from=build-env /build/app .
RUN chmod +x app

ENTRYPOINT ["/go/bin/app"]
