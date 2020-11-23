#!/bin/bash

wd=$(dirname "$0")

if [[ ! -f "$wd/build.sh" ]]; then
	echo "Build script must be executed in goproxy project directory."
	return 1
fi


function build() {
	for i in "$wd/app/"*; do
		base=$(basename "$i")
		export GOOS="$1"
		export GOARCH="$2"
		export GOARM="$3"

		ext="$GOOS.$GOARCH$GOARM"
		if [[ "$GOOS" == "windows" ]]; then
			ext="exe"
		fi
		go build -o "$wd/bin/$base.$ext" "$i/"*.go
	done
}


build windows amd64
build linux amd64  # Standard Linux
build linux arm 6  # Linux on Raspberry Pi (1) 
build linux arm 7  # Linux on Raspberry Pi (2+) 
build darwin amd64 # MacOS on Intel Hardware
