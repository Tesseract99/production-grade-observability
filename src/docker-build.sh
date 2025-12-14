#!/bin/bash

COMMIT_HASH=$(git rev-parse --short HEAD)
APP_VERSION=$(cat version.txt)

if [ -z "$COMMIT_HASH" ] || [ -z "$APP_VERSION" ]; then
    echo "Error: version.txt or git commit hash is empty"
    exit 1
fi

echo "Building image tesseract/myapp-go:v$APP_VERSION-$COMMIT_HASH"

podman build -t tesseract/myapp-go:$APP_VERSION-$COMMIT_HASH .