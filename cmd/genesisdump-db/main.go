package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	leveldb "github.com/ethereum/go-ethereum/ethdb/leveldb"
	pebble "github.com/ethereum/go-ethereum/ethdb/pebble"
)

type DumpHeader struct {
	ParentHash    string `json:"parentHash"`
	Hash          string `json:"hash"`
	StateRoot     string `json:"stateRoot"`
	Nonce         string `json:"nonce"`
	MixHash       string `json:"mixHash"`
	Timestamp     string `json:"timestamp"`
	Coinbase      string `json:"coinbase"`
	GasLimit      string `json:"gasLimit"`
	BaseFeePerGas string `json:"baseFeePerGas"`
	ExtraData     string `json:"extraData"`
	Difficulty    string `json:"difficulty"`
	Number        string `json:"number"`
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
	var base string
	flag.StringVar(&base, "chaindata", "", "path to L2 data root (e.g. ~/.arbitrum/sepolia-rollup)")
	flag.Parse()
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".arbitrum", "sepolia-rollup")
	}
	pebblePath := filepath.Join(base, "nitro", "l2chaindata")
	gethPath := filepath.Join(base, "geth", "chaindata")

	var db rawdb.DatabaseReader
	var closeFn func()

	if st, err := os.Stat(pebblePath); err == nil && st.IsDir() {
		pdb, err := pebble.New(pebblePath, 2048, 2048, "", false)
		if err != nil {
			panic(err)
		}
		db = rawdb.NewDatabase(pdb)
		closeFn = func() { pdb.Close() }
	} else {
		ldb, err := leveldb.New(gethPath, 0, 0, "", false)
		if err != nil {
			panic(err)
		}
		db = rawdb.NewDatabase(ldb)
		closeFn = func() { ldb.Close() }
	}
	defer closeFn()

	genHash := rawdb.ReadCanonicalHash(db, 0)
	if genHash == (common.Hash{}) {
		panic("no canonical hash for block 0")
	}
	block := rawdb.ReadBlock(db, genHash, 0)
	if block == nil {
		panic("failed to read block 0")
	}
	header := block.Header()
	d := Dump{
		Header: DumpHeader{
			ParentHash:    header.ParentHash.Hex(),
			Hash:          block.Hash().Hex(),
			StateRoot:     header.Root.Hex(),
			Nonce:         mustHexU64(header.Nonce.Uint64()),
			MixHash:       header.MixDigest.Hex(),
			Timestamp:     mustHexU64(header.Time),
			Coinbase:      header.Coinbase.Hex(),
			GasLimit:      mustHexU64(header.GasLimit),
			BaseFeePerGas: mustHexU256(header.BaseFee),
			ExtraData:     fmt.Sprintf("0x%x", header.Extra),
			Difficulty:    mustHexU256(header.Difficulty),
			Number:        mustHexU64(header.Number.Uint64()),
		},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(d)
}
