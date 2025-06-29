#!/bin/sh
set -e
cd "$(dirname "$0")"

# To find the symbol path for the AppVersion variable run (must have an already build binary):
#   go tool nm bin/main | grep config.AppVersion
# then set the variable bellow with the grepped path:
APP_VERSION_SYMBOL="github.com/bermr/api-golang-base/internal/config.AppVersion"

if [ "$BUILD_WITH_RACE_DETECTION" = "1" ];
then
  echo "building with race detection..."
  CGO_ENABLED=1
  go build -race -o /usr/local/bin/main -ldflags "-X $APP_VERSION_SYMBOL=$(cat ../VERSION)" ../cmd/api/main.go
else
  go build -o /usr/local/bin/main -ldflags "-X $APP_VERSION_SYMBOL=$(cat ../VERSION)" ../cmd/api/main.go
fi

echo "finished build"
