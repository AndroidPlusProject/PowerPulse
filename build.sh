#!/bin/bash

set -e
export CGO_ENABLED=1

build() {
	echo "Building PowerPulse for $GOOS $GOARCH..."
	go build -o bin/powerpulse-$GOOS-$GOARCH
	echo "Building libpowerpulse for $GOOS $GOARCH..."
	go build -o lib/libpowerpulse-$GOOS-$GOARCH.so -buildmode=c-shared -ldflags="-s -w"
	patchelf --set-soname "libpowerpulse.so" lib/libpowerpulse-$GOOS-$GOARCH.so
}

rm -rf bin
mkdir -p bin
rm -rf lib
mkdir -p lib/include

### Because of Go shenanigans, the classic file_$GOOS.go trick doesn't work for GOOS=android so we have to manage it.
set +e
mv *_linux.go linux/
mv *_android.go android/
set -e

export GOOS=linux
mv $GOOS/*_$GOOS.go ./

export CC=x86_64-linux-gnu-gcc
export GOARCH=amd64
build

export CC=i686-linux-gnu-gcc
export GOARCH=386
build

mv *_$GOOS.go $GOOS/
export GOOS=android
export CC_ROOT=$ANDROID_SDK_ROOT/ndk-bundle/toolchains/llvm/prebuilt/linux-x86_64/bin
mv $GOOS/*_$GOOS.go ./

# Ubuntu x64: gcc-arm-linux-gnueabihf libc6-dev-armhf-cross
export CC=$CC_ROOT/aarch64-linux-android30-clang
export GOARCH=arm64
build

# Ubuntu x64: gcc-aarch64-linux-gnu libc6-dev-arm64-cross
export CC=$CC_ROOT/armv7a-linux-androideabi30-clang
export GOARCH=arm
build

# Ubuntu x64: gcc-x86-64-linux-gnu
export CC=$CC_ROOT/x86_64-linux-android30-clang
export GOARCH=amd64
build

# Ubuntu x64: gcc-i686-linux-gnu libc6-dev-i686-cross
export CC=$CC_ROOT/i686-linux-android30-clang
export GOARCH=386
build

mv *_$GOOS.go $GOOS/

echo "Cleaning up headers..."
echo "* TODO: Add header arch support in Android.bp"
mv lib/libpowerpulse-android-arm64.h lib/include/libpowerpulse.h
rm lib/*.h

echo "Done building PowerPulse!"
