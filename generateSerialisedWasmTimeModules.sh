#!/bin/bash

# Check if the directory is provided
if [ -z "$1" ]; then
  echo "Usage: $0 <directory>"
  exit 1
fi

base_dir=$1

# Check if the provided directory exists
if [ ! -d "$base_dir" ]; then
  echo "Directory $base_dir does not exist."
  exit 1
fi

# Iterate over each subdirectory in the base directory
for sub_dir in "$base_dir"/*/; do
  if [ -d "$sub_dir" ]; then
    sub_dir_name=$(basename "$sub_dir")
    wasm_file="$sub_dir/$sub_dir_name.wasm"
    serialized_file="$sub_dir/$sub_dir_name.serializedwasmtime"

    echo "Running: GOOS=wasip1 GOARCH=wasm go build -o $wasm_file ./main.go in $sub_dir"
    (cd "$sub_dir" && GOOS=wasip1 GOARCH=wasm go build -o "$wasm_file" ./main.go)

    if [ -f "$wasm_file" ]; then
      echo "Serializing: $wasm_file to $serialized_file"
      go run serialize_wasm.go "$wasm_file" "$serialized_file"
    else
      echo "Failed to build $wasm_file"
    fi
  fi
done
