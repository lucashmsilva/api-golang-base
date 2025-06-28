# Build and Dev stage
FROM golang:1.24-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

RUN go install github.com/air-verse/air@latest
RUN go install golang.org/x/vuln/cmd/govulncheck@latest

COPY . .

RUN mkdir -p ./bin
RUN ./scripts/build.sh

# Release stage
FROM alpine AS release

WORKDIR /app

COPY --from=build /bin/main /main

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/main"]
