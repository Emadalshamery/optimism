#!/bin/bash

set -euo pipefail # Fail on errors, undefined vars, and pipe failures

# Get the directory of the script
dir=$(dirname "$(realpath "$0")")

# Get the target binary name from the first argument
target_binary="$dir/$1"

# Build the binaries if needed
make go-ffi > /dev/null 2>&1

# Remove the first argument
shift

# Call the target binary with remaining arguments
$target_binary "$@"
