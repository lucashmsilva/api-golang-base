#!/bin/sh
set -e

CHANGELOG=$(git log -1 --format="%s%n%b")
LAST_VERSION=$(cat ./VERSION)
NEW_VERSION=$LAST_VERSION

if echo "$CHANGELOG" | grep -q "\[BREAKING\]"; then
  NEW_VERSION=$(./scripts/increment_version.sh -M $LAST_VERSION)
elif echo "$CHANGELOG" | grep -q "\[FEATURE\]"; then
  NEW_VERSION=$(./scripts/increment_version.sh -m $LAST_VERSION)
elif echo "$CHANGELOG" | grep -q "\[PATCH\]"; then
  NEW_VERSION=$(./scripts/increment_version.sh -p $LAST_VERSION)
fi

if [ $NEW_VERSION != $LAST_VERSION ]
then
  echo $NEW_VERSION > ./VERSION
  git add VERSION
  git commit -m "Release $NEW_VERSION"
  git tag -a $NEW_VERSION -m "Release $NEW_VERSION"
  git push -f --tags origin HEAD:main
else
  echo "Warning: No changes (breaking, feature, patch) found in changelog."
  exit 1
fi
