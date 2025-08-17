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
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb"
	leveldb "github.com/ethereum/go-ethereum/ethdb/leveldb"
	pebble "github.com/ethereum/go-ethereum/ethdb/pebble"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
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

type SecureAccount struct {
	Nonce     string            `json:"nonce"`
	Balance   string            `json:"balance"`
	CodeHash  string            `json:"codeHash"`
	StorageRoot string          `json:"storageRoot"`
	Code      string            `json:"code,omitempty"`
	Storage   map[string]string `json:"storage,omitempty"`
}

type Dump struct {
	Header       DumpHeader               `json:"header"`
	SecureAlloc  map[string]SecureAccount `json:"secureAlloc,omitempty"`
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
	var dumpSecure bool
	var maxAccounts int
	var maxStorage int
	flag.StringVar(&base, "chaindata", "", "path to L2 data root (e.g. ~/.arbitrum/sepolia-rollup)")
	flag.BoolVar(&dumpSecure, "dump-secure-alloc", false, "dump secure alloc keyed by hashed addresses and storage keys")
	flag.IntVar(&maxAccounts, "max-accounts", 0, "limit number of accounts to dump (0 = no limit)")
	flag.IntVar(&maxStorage, "max-storage", 0, "limit number of storage slots per account (0 = no limit)")
	flag.Parse()
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".arbitrum", "sepolia-rollup")
	}
	pebblePath := filepath.Join(base, "nitro", "l2chaindata")
	gethPath := filepath.Join(base, "geth", "chaindata")

	var db ethdb.Database
	var closeFn func()

	if st, err := os.Stat(pebblePath); err == nil && st.IsDir() {
		pdb, err := pebble.New(pebblePath, 2048, 2048, "", false, false, nil)
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
	out := Dump{
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

	if dumpSecure {
		trieDB := trie.NewDatabase(db)
		st, err := trie.NewSecure(header.Root, trieDB)
		if err != nil {
			panic(err)
		}
		it := trie.NewIterator(st.NodeIterator(nil))
		alloc := make(map[string]SecureAccount)
		accountCount := 0
		for it.Next() {
			if len(it.Value) == 0 {
				continue
			}
			var acc state.Account
			if err := rlp.DecodeBytes(it.Value, &acc); err != nil {
				panic(err)
			}
			keyHex := "0x" + common.Bytes2Hex(it.Key)
			sacc := SecureAccount{
				Nonce:       mustHexU64(acc.Nonce),
				Balance:     mustHexU256(acc.Balance),
				CodeHash:    common.BytesToHash(acc.CodeHash[:]).Hex(),
				StorageRoot: common.BytesToHash(acc.Root[:]).Hex(),
			}
			if acc.CodeHash != (common.Hash{}) {
				code := rawdb.ReadCode(db, common.BytesToHash(acc.CodeHash[:]))
				if len(code) > 0 {
					sacc.Code = "0x" + common.Bytes2Hex(code)
				}
			}
			if acc.Root != (common.Hash{}) && acc.Root != (common.Hash{ // empty root
			}) {
				strie, err := trie.NewSecure(common.BytesToHash(acc.Root[:]), trieDB)
				if err == nil {
					sit := trie.NewIterator(strie.NodeIterator(nil))
					storage := make(map[string]string)
					stCnt := 0
					for sit.Next() {
						if len(sit.Value) == 0 {
							continue
						}
						var val common.Hash
						if err := rlp.DecodeBytes(sit.Value, &val); err != nil {
							storage["0x"+common.Bytes2Hex(sit.Key)] = "0x" + common.Bytes2Hex(sit.Value)
						} else {
							storage["0x"+common.Bytes2Hex(sit.Key)] = val.Hex()
						}
						stCnt++
						if maxStorage > 0 && stCnt >= maxStorage {
							break
						}
					}
					if len(storage) > 0 {
						sacc.Storage = storage
					}
				}
			}
			alloc[keyHex] = sacc
			accountCount++
			if maxAccounts > 0 && accountCount >= maxAccounts {
				break
			}
		}
		if len(alloc) > 0 {
			out.SecureAlloc = alloc
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
