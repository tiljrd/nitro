package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"

	"github.com/offchainlabs/nitro/arbos/arbosState"
	"github.com/offchainlabs/nitro/arbos/arbostypes"
	"github.com/offchainlabs/nitro/cmd/chaininfo"
	"github.com/ethereum/go-ethereum/core"
	"github.com/offchainlabs/nitro/statetransfer"
)

type DumpHeader struct {
	ParentHash    string `json:"parentHash"`
	Hash          string `json:"hash,omitempty"`
	StateRoot     string `json:"stateRoot,omitempty"`
	Nonce         string `json:"nonce,omitempty"`
	MixHash       string `json:"mixHash,omitempty"`
	Timestamp     string `json:"timestamp,omitempty"`
	Coinbase      string `json:"coinbase,omitempty"`
	GasLimit      string `json:"gasLimit,omitempty"`
	BaseFeePerGas string `json:"baseFeePerGas,omitempty"`
	ExtraData     string `json:"extraData,omitempty"`
	Difficulty    string `json:"difficulty,omitempty"`
}

type Dump struct {
	Header DumpHeader `json:"header"`
}

func mustHexU64(v uint64) string {
	return fmt.Sprintf("0x%x", v)
}
func mustHexU256(v *big.Int) string {
	if v == nil {
		return "0x0"
	}
	return fmt.Sprintf("0x%x", v)
}

func main() {
	chainInfo, err := chaininfo.ProcessChainInfo(421614, "sepolia-rollup", nil, "")
	if err != nil || chainInfo == nil || chainInfo.ChainConfig == nil {
		panic("failed to load sepolia-rollup chain config from chaininfo")
	}
	chainConfig := chainInfo.ChainConfig
	chainConfig.ArbitrumChainParams.GenesisBlockNum = 0

	initDataReader := statetransfer.NewMemoryInitDataReader(&statetransfer.ArbosInitializationInfo{
		ChainOwner: chainConfig.ArbitrumChainParams.InitialChainOwner,
	})

	serializedChainConfig, err := json.Marshal(chainConfig)
	if err != nil {
		panic(err)
	}
	parsedInitMessage := &arbostypes.ParsedInitMessage{
		ChainId:               chainConfig.ChainID,
		InitialL1BaseFee:      arbostypes.DefaultInitialL1BaseFee,
		ChainConfig:           chainConfig,
		SerializedChainConfig: serializedChainConfig,
	}

	db := rawdb.NewMemoryDatabase()
	cacheCfg := &core.CacheConfig{}
	stateRoot, err := arbosState.InitializeArbosInDatabase(
		db,
		cacheCfg,
		initDataReader,
		chainConfig,
		nil, // no ArbOSInit overrides
		parsedInitMessage,
		0, // timestamp for block 0
		0, // accountsPerSync
	)
	if err != nil {
		panic(err)
	}

	block := arbosState.MakeGenesisBlock(common.Hash{}, 0, 0, stateRoot, chainConfig)
	header := block.Header()

	d := Dump{
		Header: DumpHeader{
			ParentHash:    header.ParentHash.Hex(),
			Hash:          header.Hash().Hex(),
			StateRoot:     header.Root.Hex(),
			Nonce:         mustHexU64(header.Nonce.Uint64()),
			MixHash:       header.MixDigest.Hex(),
			Timestamp:     mustHexU64(header.Time),
			Coinbase:      header.Coinbase.Hex(),
			GasLimit:      mustHexU64(header.GasLimit),
			BaseFeePerGas: mustHexU256(header.BaseFee),
			ExtraData:     fmt.Sprintf("0x%x", header.Extra),
			Difficulty:    mustHexU256(header.Difficulty),
		},
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(d)
}
