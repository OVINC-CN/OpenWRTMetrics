#!/bin/bash

# build script for cross-compiling openwrt exporter

set -e

APP_NAME="openwrt-exporter"
VERSION=${VERSION:-"dev"}
BUILD_DIR="build"

# clean build directory
rm -rf ${BUILD_DIR}
mkdir -p ${BUILD_DIR}

echo "building for linux/amd64..."
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ${BUILD_DIR}/${APP_NAME}-linux-amd64

echo "building for linux/arm..."
GOOS=linux GOARCH=arm GOARM=7 go build -ldflags "-s -w" -o ${BUILD_DIR}/${APP_NAME}-linux-arm

echo "building for linux/arm64..."
GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o ${BUILD_DIR}/${APP_NAME}-linux-arm64

echo "building for linux/mips (big endian)..."
GOOS=linux GOARCH=mips go build -ldflags "-s -w" -o ${BUILD_DIR}/${APP_NAME}-linux-mips

echo "building for linux/mipsle (little endian)..."
GOOS=linux GOARCH=mipsle go build -ldflags "-s -w" -o ${BUILD_DIR}/${APP_NAME}-linux-mipsle

echo "build completed successfully!"
echo "binaries are in ${BUILD_DIR}/"
ls -lh ${BUILD_DIR}/
