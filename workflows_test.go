package load

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytecodealliance/wasmtime-go/v23"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"github.com/tetratelabs/wazero"

	wasmpb "github.com/smartcontractkit/chainlink-common/pkg/workflows/wasm/pb"
)

// For comparison with wasmtime compiling a module takes ~ 4s
func Test_Wazero_LoadAllWorkflows_NewModule_SingleThreaded(t *testing.T) {
	workflowDir := "./workflowwasmfiles_generated"
	subDirs, err := ioutil.ReadDir(workflowDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	ctx := context.Background()

	cfg := wazero.NewRuntimeConfig()

	runtime := wazero.NewRuntimeWithConfig(ctx, cfg)

	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	_, err = runtime.NewHostModuleBuilder("env").
		NewFunctionBuilder().WithFunc(func(resp *wasmpb.Response) {
		fmt.Printf("called")
	}).Export("sendResponse").Instantiate(ctx)

	for _, subDir := range subDirs {
		if subDir.IsDir() {
			subDirPath := filepath.Join(workflowDir, subDir.Name())
			files, err := ioutil.ReadDir(subDirPath)
			if err != nil {
				t.Fatalf("Failed to read subdirectory: %v", err)
			}

			for _, file := range files {
				if filepath.Ext(file.Name()) == ".wasm" {
					wasmFilePath := filepath.Join(subDirPath, file.Name())
					wasmBytes, err := os.ReadFile(wasmFilePath)
					if err != nil {
						t.Fatalf("Failed to read wasm file: %v", err)
					}

					start := time.Now()
					// Compile the Wasm module
					_, err = runtime.CompileModule(context.Background(), wasmBytes)
					if err != nil {
						t.Fatalf("Failed to compile module: %v", err)
					}

					fmt.Printf("NewModule time: %v\n", time.Since(start))
				}
			}
		}
	}
}

// Test wazero compilation in parallel, shows that parellel compilation is about same time per module, thus
// compilation can effectivley be done in parallel to reduce overall time
func Test_Wazero_LoadAllWorkflows_NewModule_MultiThreaded(t *testing.T) {
	var wg sync.WaitGroup
	numThreads := 10 // Configurable number of threads
	sem := make(chan struct{}, numThreads)

	workflowDir := "./workflowwasmfiles_generated"
	subDirs, err := ioutil.ReadDir(workflowDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	ctx := context.Background()
	cfg := wazero.NewRuntimeConfig()
	runtime := wazero.NewRuntimeWithConfig(ctx, cfg)
	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	for _, subDir := range subDirs {
		if subDir.IsDir() {
			subDirPath := filepath.Join(workflowDir, subDir.Name())
			files, err := ioutil.ReadDir(subDirPath)
			if err != nil {
				t.Fatalf("Failed to read subdirectory: %v", err)
			}

			for _, file := range files {
				if filepath.Ext(file.Name()) == ".wasm" {
					wg.Add(1)
					sem <- struct{}{}
					go func(file os.FileInfo) {
						defer wg.Done()
						defer func() { <-sem }()
						wasmFilePath := filepath.Join(subDirPath, file.Name())
						wasmBytes, err := os.ReadFile(wasmFilePath)
						if err != nil {
							t.Fatalf("Failed to read wasm file: %v", err)
						}

						start := time.Now()
						_, err = runtime.CompileModule(context.Background(), wasmBytes)
						if err != nil {
							t.Fatalf("Failed to compile module: %v", err)
						}
						fmt.Printf("NewModule time: %v\n", time.Since(start))
					}(file)
				}
			}
		}
	}
	wg.Wait()
}

// For comparison versus serialisation, shows that creating a new module is significantly slower (~50-100 times, roughly 1s) than
// deserialising a workflow module
func Test_LoadAllWorkflows_NewModule_SingleThreaded(t *testing.T) {
	workflowDir := "./workflowwasmfiles_generated"
	subDirs, err := ioutil.ReadDir(workflowDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	cfg := newCfg()
	engine := wasmtime.NewEngineWithConfig(cfg)

	for _, subDir := range subDirs {
		if subDir.IsDir() {
			subDirPath := filepath.Join(workflowDir, subDir.Name())
			files, err := ioutil.ReadDir(subDirPath)
			if err != nil {
				t.Fatalf("Failed to read subdirectory: %v", err)
			}

			for _, file := range files {
				if filepath.Ext(file.Name()) == ".wasm" {
					wasmFilePath := filepath.Join(subDirPath, file.Name())
					wasmBytes, err := os.ReadFile(wasmFilePath)
					if err != nil {
						t.Fatalf("Failed to read wasm file: %v", err)
					}

					start := time.Now()
					_, err = wasmtime.NewModule(engine, wasmBytes)
					if err != nil {
						t.Fatalf("Failed to create module: %v", err)
					}
					fmt.Printf("NewModule time: %v\n", time.Since(start))
				}
			}
		}
	}
}

// Test to check impact of running in parallel with configurable number of threads, test shows considerable degradation
// of the time taken to create a new module as new modules are created
func Test_LoadAllWorkflows_NewModule_Parallel(t *testing.T) {
	var wg sync.WaitGroup
	numThreads := 4 // Configurable number of threads
	sem := make(chan struct{}, numThreads)

	workflowDir := "./workflowwasmfiles_generated"
	subDirs, err := ioutil.ReadDir(workflowDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	cfg := newCfg()
	engine := wasmtime.NewEngineWithConfig(cfg)

	for _, subDir := range subDirs {
		if subDir.IsDir() {
			subDirPath := filepath.Join(workflowDir, subDir.Name())
			files, err := ioutil.ReadDir(subDirPath)
			if err != nil {
				t.Fatalf("Failed to read subdirectory: %v", err)
			}

			for _, file := range files {
				if filepath.Ext(file.Name()) == ".wasm" {
					wg.Add(1)
					sem <- struct{}{}
					go func(file os.FileInfo) {
						defer wg.Done()
						defer func() { <-sem }()
						wasmFilePath := filepath.Join(subDirPath, file.Name())
						wasmBytes, err := os.ReadFile(wasmFilePath)
						if err != nil {
							t.Fatalf("Failed to read wasm file: %v", err)
						}

						start := time.Now()
						_, err = wasmtime.NewModule(engine, wasmBytes)
						if err != nil {
							t.Fatalf("Failed to create module: %v", err)
						}
						fmt.Printf("NewModule time: %v\n", time.Since(start))
					}(file)
				}
			}
		}
	}
	wg.Wait()
}

// Test to compare relative performance of deserializing from file versus new module - demonstrates the but where deserializtion
// rapidly slows down on the 2nd call
func Test_LoadAllWorkflows_DeserializeFromFile_SingleThreaded(t *testing.T) {
	deserializeSingleThreaded(t, 100)
}

func deserializeSingleThreaded(t *testing.T, maxToLoad int) {
	workflowDir := "./workflowwasmfiles_generated"
	subDirs, err := ioutil.ReadDir(workflowDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	cfg := newCfg()
	engine := wasmtime.NewEngineWithConfig(cfg)

	loaded := 0
	for _, subDir := range subDirs {
		if subDir.IsDir() {
			subDirPath := filepath.Join(workflowDir, subDir.Name())
			files, err := ioutil.ReadDir(subDirPath)
			if err != nil {
				t.Fatalf("Failed to read subdirectory: %v", err)
			}

			for _, file := range files {
				if filepath.Ext(file.Name()) == ".serializedwasmtime" {
					serializedFilePath := filepath.Join(subDirPath, file.Name())

					start := time.Now()
					_, err := wasmtime.NewModuleDeserializeFile(engine, serializedFilePath)
					if err != nil {
						t.Fatalf("Failed to deserialize module: %v", err)
					}
					fmt.Printf("DeserializeFromFile time: %v\n", time.Since(start))
					loaded++
					if loaded >= maxToLoad {
						return
					}
				}
			}
		}
	}
}

// This test demonstrates the bug whereby deserialization is initially fast but on the second call it becomes very slow
func Test_LoadAllWorkflows_DeserializeFromFile_Parallel(t *testing.T) {
	// First time all is well, deserialization is relatively fast
	fmt.Printf("RUNNING in parallel first time\n")
	deserializeFromFileParallel(t, 20)

	// Second time deserialization is very slow
	fmt.Printf("RUNNING in parallel second time\n")
	deserializeFromFileParallel(t, 20)
}

func deserializeFromFileParallel(t *testing.T, maxToLoad int) {
	var loaded int64
	var wg sync.WaitGroup
	numThreads := 4 // Configurable number of threads
	sem := make(chan struct{}, numThreads)

	workflowDir := "./workflowwasmfiles_generated"
	subDirs, err := ioutil.ReadDir(workflowDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	cfg := newCfg()
	engine := wasmtime.NewEngineWithConfig(cfg)

	for _, subDir := range subDirs {
		if subDir.IsDir() {
			subDirPath := filepath.Join(workflowDir, subDir.Name())
			files, err := ioutil.ReadDir(subDirPath)
			if err != nil {
				t.Fatalf("Failed to read subdirectory: %v", err)
			}

			for _, file := range files {
				if filepath.Ext(file.Name()) == ".serializedwasmtime" {
					wg.Add(1)
					sem <- struct{}{}
					go func(file os.FileInfo) {
						defer wg.Done()
						defer func() { <-sem }()
						serializedFilePath := filepath.Join(subDirPath, file.Name())

						start := time.Now()
						_, err := wasmtime.NewModuleDeserializeFile(engine, serializedFilePath)
						if err != nil {
							t.Fatalf("Failed to deserialize module: %v", err)
						}
						fmt.Printf("DeserializeFromFile time: %v\n", time.Since(start))
						newLoaded := atomic.AddInt64(&loaded, 1)
						if newLoaded >= int64(maxToLoad) {
							return
						}
					}(file)
				}
			}
		}
	}
	wg.Wait()
}

// This test reliably demonstrates the slowdown of deserialization observed in the core, even
// though it loads no more than about 90 modules, which is much less than that than can be loaded
// by the load in single thread test, the time taken to load the modules slows down significantly
func Test_LoadAllWorkflows_DeserializeFromFile_ParallelWithLock(t *testing.T) {
	fmt.Printf("RUNNING in parallel with lock 1st time\n")
	testRunInParalellWithLock(t, 30)

	fmt.Printf("RUNNING in parallel with lock 2nd time\n")
	testRunInParalellWithLock(t, 30)

	fmt.Printf("RUNNING in parallel with lock 3rd time\n")
	testRunInParalellWithLock(t, 30)
}

func testRunInParalellWithLock(t *testing.T, maxToLoad int64) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	numThreads := 4 // Configurable number of threads
	sem := make(chan struct{}, numThreads)

	workflowDir := "./workflowwasmfiles_generated"
	subDirs, err := ioutil.ReadDir(workflowDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	var counter int64

	for _, subDir := range subDirs {
		if subDir.IsDir() {
			subDirPath := filepath.Join(workflowDir, subDir.Name())
			files, err := ioutil.ReadDir(subDirPath)
			if err != nil {
				t.Fatalf("Failed to read subdirectory: %v", err)
			}

			for _, file := range files {

				if filepath.Ext(file.Name()) == ".serializedwasmtime" {
					wg.Add(1)
					sem <- struct{}{}
					go func(file os.FileInfo) {
						defer wg.Done()
						defer func() { <-sem }()

						loaded := atomic.LoadInt64(&counter)
						if loaded >= maxToLoad {
							return
						}

						serializedFilePath := filepath.Join(subDirPath, file.Name())

						cfg := newCfg()

						engine := wasmtime.NewEngineWithConfig(cfg)

						mu.Lock()
						start := time.Now()
						_, err = wasmtime.NewModuleDeserializeFile(engine, serializedFilePath)
						if err != nil {
							t.Fatalf("Failed to deserialize module: %v", err)
						}
						fmt.Printf("DeserializeFromFile time: %v   count: %d file: %s\n", time.Since(start), atomic.LoadInt64(&counter), file.Name())
						atomic.AddInt64(&counter, 1)

						mu.Unlock()
					}(file)
				}
			}
		}
	}
	wg.Wait()
}

// sets up the config to match that used in core
func newCfg() *wasmtime.Config {
	cfg := wasmtime.NewConfig()
	cfg.SetEpochInterruption(true)
	cfg.CacheConfigLoadDefault()
	cfg.SetCraneliftOptLevel(wasmtime.OptLevelSpeedAndSize)
	return cfg
}

// A test to confirm all binaries are unique
func Test_LoadAllWorkflows_UniqueIBinaries_SingleThreaded(t *testing.T) {
	workflowDir := "./workflowwasmfiles_generated"
	subDirs, err := ioutil.ReadDir(workflowDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	ids := make(map[string]struct{})

	for _, subDir := range subDirs {
		if subDir.IsDir() {
			subDirPath := filepath.Join(workflowDir, subDir.Name())
			files, err := ioutil.ReadDir(subDirPath)
			if err != nil {
				t.Fatalf("Failed to read subdirectory: %v", err)
			}

			for _, file := range files {
				if filepath.Ext(file.Name()) == ".wasm" {
					wasmFilePath := filepath.Join(subDirPath, file.Name())
					wasmBytes, err := os.ReadFile(wasmFilePath)
					if err != nil {
						t.Fatalf("Failed to read wasm file: %v", err)
					}

					hash := sha256.Sum256(wasmBytes)
					id := fmt.Sprintf("%x", hash[:])

					if _, exists := ids[id]; exists {
						t.Fatalf("Duplicate binary found: %v", id)
					}
					ids[id] = struct{}{}
				}
			}
		}
	}
}
