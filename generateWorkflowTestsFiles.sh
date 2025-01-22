#!/bin/bash

# Check if the number of directories to create is provided
if [ -z "$1" ]; then
  echo "Usage: $0 <number_of_directories>"
  exit 1
fi



# Number of directories to create
num_dirs=$1

# Generate workflow Go files
./scripts/generateWorkflowGoFiles.sh "$num_dirs"

# Compile Go files to WebAssembly
./scripts/generateWasmFiles.sh "workflowwasmfiles_generated"

# Serialize the generated WebAssembly files
./generateSerialisedWasmTimeModules.sh "workflowwasmfiles_generated"
