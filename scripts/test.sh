#!/bin/sh
set -e

cd "$(dirname "$0")"

go test ../... -v -coverprofile=../.coverage.out

if [ "$1" = "-c" ];
then
  echo "generating coverage report..."
  go tool cover -html=../.coverage.out -o ../.coverage.html
  echo "report written to .coverage.html"
fi
