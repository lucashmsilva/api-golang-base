#!/bin/sh
set -e
cd "$(dirname "$0")"

# To find the symbol path for the AppVersion variable run (must have an already build binary):
#   go tool nm bin/main | grep config.AppVersion
# then set the variable bellow with the grepped path:
APP_NAME_SYMBOL="github.com/bermr/api-golang-base/internal/config.AppVersion"

go build -o ../bin/main -ldflags "-X $APP_NAME_SYMBOL=$(cat ../VERSION)" ../cmd/api/main.go
