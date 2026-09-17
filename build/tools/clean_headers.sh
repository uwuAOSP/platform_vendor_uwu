#!/bin/bash
set -eu

export ANDROID_BUILD_TOP="$PWD"

# clean_header.py resolves relative paths against its own 'original' tree, so
# pass absolute paths to clean the generated kernel headers in place.
headers_root="$(realpath "$1")"

./bionic/libc/kernel/tools/clean_header.py -u \
    "$headers_root/usr/include/asm/signal.h" \
    "$headers_root/usr/include/asm-generic/signal.h"
