# Build and Dev stage
FROM golang:1.24-alpine AS build

ARG BUILD_WITH_RACE_DETECTION="0"
ENV BUILD_WITH_RACE_DETECTION=$BUILD_WITH_RACE_DETECTION

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

RUN if [ "$BUILD_WITH_RACE_DETECTION" = "1" ]; then apk add build-base; fi

RUN go install github.com/air-verse/air@latest
RUN go install golang.org/x/vuln/cmd/govulncheck@latest

COPY . .

RUN mkdir -p /usr/local/bin/
RUN ./scripts/build.sh

# Release stage
FROM alpine AS release

WORKDIR /app

COPY --from=build /usr/local/bin/main ./main

EXPOSE 8080

ENTRYPOINT ["./main"]
