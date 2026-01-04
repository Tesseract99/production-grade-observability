#!/bin/bash

COMMIT_HASH=$(git rev-parse --short HEAD)
APP_VERSION=$(cat version.txt)
IMAGE_NAME="tesseract/myapp-go:v$APP_VERSION-$COMMIT_HASH"


if [ -z "$COMMIT_HASH" ] || [ -z "$APP_VERSION" ]; then
    echo "Error: version.txt or git commit hash is empty"
    exit 1
fi

echo "Building image $IMAGE_NAME"

podman build -t $IMAGE_NAME .


# export to kind cluster
podman save -o myapp-go-$COMMIT_HASH.tar localhost/$IMAGE_NAME

kind load image-archive myapp-go-$COMMIT_HASH.tar --name prithvi-cluster

rm myapp-go-$COMMIT_HASH.tar