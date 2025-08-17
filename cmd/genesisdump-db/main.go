package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"bytes"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	leveldb "github.com/ethereum/go-ethereum/ethdb/leveldb"
	pebble "github.com/ethereum/go-ethereum/ethdb/pebble"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/triedb"
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
	Header      DumpHeader               `json:"header"`
	SecureAlloc map[string]SecureAccount `json:"secureAlloc,omitempty"`
	Alloc       map[string]SecureAccount `json:"alloc,omitempty"`
}

func hexToAddress(s string) common.Address {
	b := common.FromHex(s)
	var out common.Address
	if len(b) == 20 {
		copy(out[:], b)
	}
	return out
}

func keccakAddress(addr common.Address) common.Hash {
	return crypto.Keccak256Hash(addr[:])
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
		},
	}

	if dumpSecure {
		tdb := triedb.NewDatabase(db, nil)
		atrie, err := trie.NewStateTrie(trie.StateTrieID(header.Root), tdb)
		if err != nil {
			panic(err)
		}
		it, err := atrie.NodeIterator(nil)
		if err != nil {
			panic(err)
		}
		ni := trie.NewIterator(it)
		secureAlloc := make(map[string]SecureAccount)
		plainAlloc := make(map[string]SecureAccount)
		accountCount := 0
		for ni.Next() {
			if len(ni.Value) == 0 {
				continue
			}
			var acc types.StateAccount
			if err := rlp.DecodeBytes(ni.Value, &acc); err != nil {
				panic(err)
			}
			hashedKey := common.BytesToHash(ni.Key)
			keyHex := "0x" + common.Bytes2Hex(ni.Key)

			sacc := SecureAccount{
				Nonce:       mustHexU64(acc.Nonce),
				Balance:     mustHexU256(acc.Balance.ToBig()),
				CodeHash:    common.BytesToHash(acc.CodeHash).Hex(),
				StorageRoot: acc.Root.Hex(),
			}
			if !bytes.Equal(acc.CodeHash, types.EmptyCodeHash[:]) {
				code := rawdb.ReadCode(db, common.BytesToHash(acc.CodeHash))
				if len(code) > 0 {
					sacc.Code = "0x" + common.Bytes2Hex(code)
				}
			}

			var storageSecure map[string]string
			var storagePlain map[string]string
			if acc.Root != (common.Hash{}) {
				addrHash := hashedKey // hashed address key from secure trie
				strie, err := trie.NewStateTrie(trie.StorageTrieID(header.Root, addrHash, acc.Root), tdb)
				if err == nil {
					sit, err := strie.NodeIterator(nil)
					if err == nil {
						siter := trie.NewIterator(sit)
						storageSecure = make(map[string]string)
						storagePlain = make(map[string]string)
						stCnt := 0
						for siter.Next() {
							if len(siter.Value) == 0 {
								continue
							}
							var bi big.Int
							valHex := ""
							if err := rlp.DecodeBytes(siter.Value, &bi); err != nil {
								valHex = "0x" + common.Bytes2Hex(siter.Value)
							} else {
								valHex = fmt.Sprintf("0x%x", &bi)
							}
							secureKeyHex := "0x" + common.Bytes2Hex(siter.Key)
							storageSecure[secureKeyHex] = valHex
							slotPreimage := rawdb.ReadPreimage(db, common.BytesToHash(siter.Key))
							if len(slotPreimage) == 32 {
								storagePlain["0x"+common.Bytes2Hex(slotPreimage)] = valHex
							}
							stCnt++
							if maxStorage > 0 && stCnt >= maxStorage {
								break
							}
						}
					}
				}
			}
			if len(storageSecure) > 0 {
				sacc.Storage = storageSecure
			}

			secureAlloc[keyHex] = sacc

			addrPreimage := rawdb.ReadPreimage(db, hashedKey)
			if len(addrPreimage) == 20 {
				pacc := sacc
				if len(storagePlain) > 0 {
					pacc.Storage = storagePlain
				}
				plainAlloc["0x"+common.Bytes2Hex(addrPreimage)] = pacc
			}

			accountCount++
			if maxAccounts > 0 && accountCount >= maxAccounts {
				break
			}
		}
		if len(secureAlloc) > 0 {
			out.SecureAlloc = secureAlloc
		}
		if len(plainAlloc) > 0 {
		if len(out.SecureAlloc) > 0 {
			fmt.Fprintln(os.Stderr, "dbg: secureAlloc keys (first 5):")
			i := 0
			for k := range out.SecureAlloc {
				fmt.Fprintln(os.Stderr, "  secureKey:", k)
				i++
				if i >= 5 {
					break
				}
			}
		}
		fmt.Fprintln(os.Stderr, "dbg: candidate addr -> keccak(addr) (first few):")
		sampleCandidates := []common.Address{
			hexToAddress("0x0000000000000000000000000000000000000001"),
			hexToAddress("0x0000000000000000000000000000000000000002"),
			hexToAddress("0x0000000000000000000000000000000000000003"),
			hexToAddress("0x0000000000000000000000000000000000000004"),
			hexToAddress("0x0000000000000000000000000000000000000005"),
			hexToAddress("0x0000000000000000000000000000000000000064"),
			hexToAddress("0x0000000000000000000000000000000000000066"),
			hexToAddress("0x000000000000000000000000000000000000006c"),
			hexToAddress("0x000000000000000000000000000000000000006e"),
			hexToAddress("0x0000000000000000000000000000000000000070"),
			hexToAddress("0x00000000000000000000000000000000000000c8"),
		}
		for _, a := range sampleCandidates {
			if (a != common.Address{}) {
				fmt.Fprintln(os.Stderr, "  ", a.Hex(), "->", keccakAddress(a).Hex())
			}
		}
			out.Alloc = plainAlloc
		}
		if (out.Alloc == nil || len(out.Alloc) == 0) && len(out.SecureAlloc) > 0 {
			hashToAddr := make(map[common.Hash]common.Address, 256)
			for i := 1; i <= 0xff; i++ {
				addrBytes := make([]byte, 20)
				addrBytes[19] = byte(i)
				var a common.Address
				copy(a[:], addrBytes)
				h := keccakAddress(a)
				hashToAddr[h] = a
			}
			for _, hexStr := range []string{
				"0x0000000000000000000000000000000000000064",
				"0x0000000000000000000000000000000000000065",
				"0x0000000000000000000000000000000000000066",
				"0x000000000000000000000000000000000000006c",
				"0x000000000000000000000000000000000000006e",
				"0x0000000000000000000000000000000000000070",
				"0x00000000000000000000000000000000000000c8",
			} {
				a := hexToAddress(hexStr)
				if (a != common.Address{}) {
					hashToAddr[keccakAddress(a)] = a
				}
			}
			plainAlloc := make(map[string]SecureAccount)
			mapped := 0
			for k, v := range out.SecureAlloc {
				h := common.HexToHash(k)
				if addr, ok := hashToAddr[h]; ok {
					plainAlloc[addr.Hex()] = v
					mapped++
				}
			}
			if len(plainAlloc) > 0 && mapped == len(out.SecureAlloc) {
				out.Alloc = plainAlloc
			}
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
