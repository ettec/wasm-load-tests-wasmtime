#!/bin/bash



# Check if the number of directories to create is provided
if [ -z "$1" ]; then
  echo "Usage: $0 <number_of_directories>"
  exit 1
fi


# Number of directories to create
num_dirs=$1


mkdir -p helloworldwasmfiles_generated

cd helloworldwasmfiles_generated

# Base directory name
base_dir="helloworld"

# Create directories and main.go files
for ((i=1; i<=num_dirs; i++)); do
  dir_name="${base_dir}${i}"
  mkdir -p "$dir_name"
  cat <<EOL > "$dir_name/main.go"
package main

import (
  "fmt"
)

func main() {
  fmt.Printf("Hello, World${i}!\n")
}
EOL
  echo "Created directory $dir_name with main.go"
done


cd ..
