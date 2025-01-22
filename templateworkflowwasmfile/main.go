package main

import (
	"encoding/hex"
	"encoding/json"
	"math/big"

	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/smartcontractkit/chainlink-common/pkg/workflows/wasm"

	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/consensus/ocr3/aggregators"
	"github.com/smartcontractkit/chainlink-common/pkg/capabilities/consensus/ocr3/ocr3cap"
	"github.com/smartcontractkit/chainlink-common/pkg/workflows/sdk"
)

type computeOutput struct {
	Price     *big.Int
	FeedID    [32]byte
	Timestamp int64
}

type computeConfig struct {
	FeedID string
}

func convertFeedIDtoBytes(feedIDStr string) ([32]byte, error) {
	b, err := hex.DecodeString(feedIDStr[2:])
	if err != nil {
		return [32]byte{}, err
	}

	if len(b) < 32 {
		nb := [32]byte{}
		copy(nb[:], b[:])
		return nb, err
	}

	return [32]byte(b), nil
}

type readBalancesConfig struct {
	ChainID                      string `json:"chainId"`
	BalanceReaderContractAddress string
	ConsumerAddress              string
	WriteTargetCapabilityID      string
	Addresses                    []string
	CronSchedule                 string
}

func BuildWorkflow(configBytes []byte) (*sdk.WorkflowSpecFactory, error) {

	config := &readBalancesConfig{}
	if err := json.Unmarshal(configBytes, config); err != nil {
		return nil, fmt.Errorf("could not unmarshal config: %w", err)
	}

	workflow := sdk.NewWorkflowSpecFactory()

	var addresses []common.Address
	for _, addr := range config.Addresses {
		addresses = append(addresses, common.HexToAddress(addr))
	}

	// https://sepolia.etherscan.io/address/0x93c4bB995e7B5a726c8ef1bED9EA92e300F18eb4

	compConf := computeConfig{
		FeedID: "0x02ce8bfc980000320000000000000000",
	}

	compute := sdk.Compute1WithConfig(
		workflow,
		"compute",
		&sdk.ComputeConfig[computeConfig]{Config: compConf},
		sdk.Compute1Inputs[[]any]{},
		func(runtime sdk.Runtime, config computeConfig, outputs []any) (computeOutput, error) {
			feedID, err := convertFeedIDtoBytes(config.FeedID)
			if err != nil {
				return computeOutput{}, fmt.Errorf("cannot convert feedID to bytes")
			}

			balances := outputs

			balance := &big.Int{}
			for _, bal := range balances {
				bi, ok := bal.(*big.Int)
				if !ok {
					return computeOutput{}, fmt.Errorf("cannot convert value to *big.Int, got %T", bi)
				}

				balance = balance.Add(balance, bi)
			}

			return computeOutput{
				Price:     balance,
				FeedID:    feedID, // Randomly generated
				Timestamp: time.Now().Unix(),
			}, nil
		},
	)

	consensusInput := ocr3cap.ReduceConsensusInput[computeOutput]{
		Observation: compute.Value(),
	}

	consensus := ocr3cap.ReduceConsensusConfig[computeOutput]{
		Encoder: ocr3cap.EncoderEVM,
		EncoderConfig: map[string]any{
			"abi": "(bytes32 FeedID, uint224 Price, uint32 Timestamp)[] Reports",
		},
		ReportID: "0001",
		KeyID:    "evm",
		AggregationConfig: aggregators.ReduceAggConfig{
			Fields: []aggregators.AggregationField{
				{
					InputKey:  "FeedID",
					OutputKey: "FeedID",
					Method:    "mode",
				},
				{
					InputKey:        "Price",
					OutputKey:       "Price",
					Method:          "median",
					DeviationString: "1",
					DeviationType:   "percent",
				},
				{
					InputKey:        "Timestamp",
					OutputKey:       "Timestamp",
					Method:          "median",
					DeviationString: "0", // 5 minutes
					DeviationType:   "absolute",
				},
			},
			ReportFormat: aggregators.REPORT_FORMAT_ARRAY,
		},
	}.New(workflow, "consensus", consensusInput)

	fmt.Printf("Consensus: %v\n", consensus)

	return workflow, nil
}

func main() {
	runner := wasm.NewRunner()
	workflow, err := BuildWorkflow(runner.Config())
	if err != nil {
		panic(fmt.Errorf("could not build workflow: %w", err))
	}

	runner.Run(workflow)
}
