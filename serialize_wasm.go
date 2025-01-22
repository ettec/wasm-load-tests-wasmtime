package main

import (
	"fmt"
	"os"

	"github.com/bytecodealliance/wasmtime-go/v23"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: serialize_wasm <wasm_file> <output_file>")
		os.Exit(1)
	}

	wasmFile := os.Args[1]
	outputFile := os.Args[2]

	wasmBytes, err := os.ReadFile(wasmFile)
	if err != nil {
		panic(err)
	}

	cfg := wasmtime.NewConfig()
	cfg.SetEpochInterruption(true)
	cfg.CacheConfigLoadDefault()
	cfg.SetCraneliftOptLevel(wasmtime.OptLevelSpeedAndSize)
	engine := wasmtime.NewEngineWithConfig(cfg)
	mod, err := wasmtime.NewModule(engine, wasmBytes)
	if err != nil {
		panic(err)
	}

	serialized, err := mod.Serialize()
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(outputFile, serialized, 0644)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Serialized module written to %s\n", outputFile)
}
