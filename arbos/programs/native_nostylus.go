 //go:build !wasm && nostylus

package programs

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/offchainlabs/nitro/arbos/burn"
	"github.com/offchainlabs/nitro/arbos/util"
)

func activateProgram(
	db vm.StateDB,
	program common.Address,
	codehash common.Hash,
	wasm []byte,
	page_limit uint16,
	stylusVersion uint16,
	arbosVersionForGas uint64,
	debug bool,
	burner burn.Burner,
	runCtx *core.MessageRunContext,
) (*activationInfo, error) {
	return &activationInfo{}, nil
}

func activateProgramInternal(
	addressForLogging common.Address,
	codehash common.Hash,
	wasm []byte,
	page_limit uint16,
	stylusVersion uint16,
	arbosVersionForGas uint64,
	debug bool,
	gasLeft *uint64,
	targets []rawdb.WasmTarget,
	moduleActivationMandatory bool,
) (*activationInfo, map[rawdb.WasmTarget][]byte, error) {
	return &activationInfo{}, map[rawdb.WasmTarget][]byte{}, nil
}

func cacheProgram(
	db vm.StateDB,
	module common.Hash,
	program Program,
	addressForLogging common.Address,
	code []byte,
	codehash common.Hash,
	params *StylusParams,
	debug bool,
	time uint64,
	runCtx *core.MessageRunContext,
) {
}

func evictProgram(
	db vm.StateDB,
	module common.Hash,
	version uint16,
	debug bool,
	runCtx *core.MessageRunContext,
	forever bool,
) {
}

func getCompiledProgram(
	statedb vm.StateDB,
	moduleHash common.Hash,
	addressForLogging common.Address,
	code []byte,
	codehash common.Hash,
	maxWasmSize uint32,
	pagelimit uint16,
	time uint64,
	debugMode bool,
	program Program,
	runCtx *core.MessageRunContext,
) (map[rawdb.WasmTarget][]byte, error) {
	return map[rawdb.WasmTarget][]byte{}, nil
}

func callProgram(
	address common.Address,
	moduleHash common.Hash,
	localAsm []byte,
	scope *vm.ScopeContext,
	interpreter *vm.EVMInterpreter,
	tracingInfo *util.TracingInfo,
	calldata []byte,
	evmData *EvmData,
	stylusParams *ProgParams,
	memoryModel *MemoryModel,
	runCtx *core.MessageRunContext,
) ([]byte, error) {
	return nil, nil
}
