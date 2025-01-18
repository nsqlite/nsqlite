#!/bin/bash
set -e

OUTPUT_DIR="./dist"
mkdir -p "$OUTPUT_DIR"

# Define build configurations: GOOS|GOARCH|ZIG_TARGET|EXT
TARGETS=(
  "linux|amd64|x86_64-linux-gnu|"
  "linux|arm64|aarch64-linux-gnu|"
  "windows|amd64|x86_64-windows-gnu|.exe"
  "windows|arm64|aarch64-windows-gnu|.exe"

  # TODO: Fix macos build
  # "darwin|amd64|x86_64-macos|"
  # "darwin|arm64|aarch64-macos|"
)

COMMANDS=("nsqlited" "nsqlite")

for TARGET in "${TARGETS[@]}"; do
  IFS='|' read -r GOOS GOARCH ZIG_TARGET EXT <<< "$TARGET"
  
  export CGO_ENABLED=1
  export GOOS
  export GOARCH
  export CC="zig cc -target $ZIG_TARGET"
  export CXX="zig c++ -target $ZIG_TARGET"

  for CMD in "${COMMANDS[@]}"; do
    BIN_NAME="${CMD}_${GOOS}_${GOARCH}${EXT}"
    echo "Building $BIN_NAME..."
    go build -o "$OUTPUT_DIR/$BIN_NAME" "./cmd/$CMD/."
  done
done

echo "Build completed. Binaries are in the $OUTPUT_DIR directory."
