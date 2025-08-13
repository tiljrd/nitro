package rethexec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethstate "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/log"

	"github.com/offchainlabs/nitro/arbos/arbostypes"
	"github.com/offchainlabs/nitro/arbutil"
	"github.com/offchainlabs/nitro/execution"
	"github.com/offchainlabs/nitro/util/containers"
)

type ConfigFetcher func() *Config

type RethExecutionClient struct {
	cfgFetcher ConfigFetcher
	rpc        *rpcClient
	started    bool
}

func NewRethExecutionClient(cfgFetcher ConfigFetcher) (*RethExecutionClient, error) {
	cfg := cfgFetcher()
	if cfg.URL == "" {
		return nil, errors.New("reth rpc url must be provided")
	}
	urls := append([]string{cfg.URL}, cfg.Secondary...)
	var jwtSecret []byte
	if cfg.JWTSecretPath != "" {
		if data, err := os.ReadFile(cfg.JWTSecretPath); err == nil {
			jwtSecret = bytesTrimSpace(data)
		} else {
			log.Warn("failed reading JWT secret, continuing without it", "path", cfg.JWTSecretPath, "err", err)
		}
	}
	rpc := newRPCClient(urls, time.Duration(cfg.TimeoutSeconds)*time.Second, jwtSecret)
	return &RethExecutionClient{cfgFetcher: cfgFetcher, rpc: rpc}, nil
}

func bytesTrimSpace(b []byte) []byte {
	i := 0
	j := len(b)
	for i < j && (b[i] == ' ' || b[i] == '\n' || b[i] == '\r' || b[i] == '\t') {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\n' || b[j-1] == '\r' || b[j-1] == '\t') {
		j--
	}
	return b[i:j]
}

func (c *RethExecutionClient) Start(ctx context.Context) error {
	c.started = true
	return nil
}

func (c *RethExecutionClient) StopAndWait() {
	c.started = false
}

func (c *RethExecutionClient) MarkFeedStart(to arbutil.MessageIndex) containers.PromiseInterface[struct{}] {
	return containers.NewReadyPromise(struct{}{}, nil)
}

func (c *RethExecutionClient) Initialize(ctx context.Context) error {
	return nil
}

func (c *RethExecutionClient) DigestMessage(msgIdx arbutil.MessageIndex, msg *arbostypes.MessageWithMetadata, msgForPrefetch *arbostypes.MessageWithMetadata) containers.PromiseInterface[*execution.MessageResult] {
	type digestParams struct {
		MsgIndex uint64                          `json:"msgIndex"`
		Msg      arbostypes.MessageWithMetadata  `json:"msg"`
	}
	type digestResult struct {
		BlockHash common.Hash `json:"blockHash"`
		SendRoot  common.Hash `json:"sendRoot"`
	}

	dp := digestParams{MsgIndex: uint64(msgIdx), Msg: *msg}
	var out digestResult
	err := c.rpc.do(context.Background(), "arb_digestMessage", []interface{}{dp}, &out)
	var res *execution.MessageResult
	if err == nil {
		res = &execution.MessageResult{BlockHash: out.BlockHash, SendRoot: out.SendRoot}
	}
	return containers.NewReadyPromise(res, err)
}

func (c *RethExecutionClient) Reorg(msgIdxOfFirstMsgToAdd arbutil.MessageIndex, newMessages []arbostypes.MessageWithMetadataAndBlockInfo, oldMessages []*arbostypes.MessageWithMetadata) containers.PromiseInterface[[]*execution.MessageResult] {
	type reorgParams struct {
		FirstIdx    uint64                                        `json:"firstIdx"`
		NewMessages []arbostypes.MessageWithMetadataAndBlockInfo  `json:"newMessages"`
		OldMessages []*arbostypes.MessageWithMetadata             `json:"oldMessages"`
	}
	var out []execution.MessageResult
	params := reorgParams{FirstIdx: uint64(msgIdxOfFirstMsgToAdd), NewMessages: newMessages, OldMessages: oldMessages}
	err := c.rpc.do(context.Background(), "arb_reorg", []interface{}{params}, &out)
	ptrs := make([]*execution.MessageResult, len(out))
	for i := range out {
		elem := out[i]
		ptrs[i] = &elem
	}
	return containers.NewReadyPromise(ptrs, err)
}

func (c *RethExecutionClient) HeadMessageIndex() containers.PromiseInterface[arbutil.MessageIndex] {
	var out uint64
	err := c.rpc.do(context.Background(), "arb_headMessageIndex", []interface{}{}, &out)
	return containers.NewReadyPromise(arbutil.MessageIndex(out), err)
}

func (c *RethExecutionClient) ResultAtMessageIndex(msgIdx arbutil.MessageIndex) containers.PromiseInterface[*execution.MessageResult] {
	type resultRes struct {
		BlockHash common.Hash `json:"blockHash"`
		SendRoot  common.Hash `json:"sendRoot"`
	}
	var out resultRes
	err := c.rpc.do(context.Background(), "arb_resultAtMessageIndex", []interface{}{uint64(msgIdx)}, &out)
	var res *execution.MessageResult
	if err == nil {
		res = &execution.MessageResult{BlockHash: out.BlockHash, SendRoot: out.SendRoot}
	}
	return containers.NewReadyPromise(res, err)
}

func (c *RethExecutionClient) MessageIndexToBlockNumber(messageNum arbutil.MessageIndex) containers.PromiseInterface[uint64] {
	var out uint64
	err := c.rpc.do(context.Background(), "arb_messageIndexToBlockNumber", []interface{}{uint64(messageNum)}, &out)
	return containers.NewReadyPromise(out, err)
}

func (c *RethExecutionClient) BlockNumberToMessageIndex(blockNum uint64) containers.PromiseInterface[arbutil.MessageIndex] {
	var out uint64
	err := c.rpc.do(context.Background(), "arb_blockNumberToMessageIndex", []interface{}{blockNum}, &out)
	return containers.NewReadyPromise(arbutil.MessageIndex(out), err)
}

func (c *RethExecutionClient) SetFinalityData(ctx context.Context, safeFinalityData *arbutil.FinalityData, finalizedFinalityData *arbutil.FinalityData, validatedFinalityData *arbutil.FinalityData) containers.PromiseInterface[struct{}] {
	type finParams struct {
		Safe       *arbutil.FinalityData `json:"safe"`
		Finalized  *arbutil.FinalityData `json:"finalized"`
		Validated  *arbutil.FinalityData `json:"validated"`
	}
	_ = c.rpc.do(ctx, "arb_setFinalityData", []interface{}{finParams{Safe: safeFinalityData, Finalized: finalizedFinalityData, Validated: validatedFinalityData}}, nil)
	return containers.NewReadyPromise(struct{}{}, nil)
}

func (c *RethExecutionClient) MarkValid(pos arbutil.MessageIndex, resultHash common.Hash) {
}

func (c *RethExecutionClient) PrepareForRecord(ctx context.Context, start, end arbutil.MessageIndex) error {
	return nil
}

func (c *RethExecutionClient) RecordBlockCreation(ctx context.Context, pos arbutil.MessageIndex, msg *arbostypes.MessageWithMetadata) (*execution.RecordResult, error) {
	type recParams struct {
		Pos uint64                         `json:"pos"`
		Msg arbostypes.MessageWithMetadata `json:"msg"`
	}
	var out struct {
		Pos       uint64                         `json:"pos"`
		BlockHash common.Hash                    `json:"blockHash"`
		Preimages map[common.Hash][]byte         `json:"preimages"`
		UserWasms gethstate.UserWasms            `json:"userWasms"`
	}
	err := c.rpc.do(ctx, "arb_recordBlockCreation", []interface{}{recParams{Pos: uint64(pos), Msg: *msg}}, &out)
	if err != nil {
		return nil, err
	}
	return &execution.RecordResult{
		Pos:       arbutil.MessageIndex(out.Pos),
		BlockHash: out.BlockHash,
		Preimages: out.Preimages,
		UserWasms: out.UserWasms,
	}, nil
}

func (c *RethExecutionClient) TriggerMaintenance() containers.PromiseInterface[struct{}] {
	_ = c.rpc.do(context.Background(), "arb_triggerMaintenance", []interface{}{}, nil)
	return containers.NewReadyPromise(struct{}{}, nil)
}

func (c *RethExecutionClient) ShouldTriggerMaintenance() containers.PromiseInterface[bool] {
	var out bool
	_ = c.rpc.do(context.Background(), "arb_shouldTriggerMaintenance", []interface{}{}, &out)
	return containers.NewReadyPromise(out, nil)
}

func (c *RethExecutionClient) MaintenanceStatus() containers.PromiseInterface[*execution.MaintenanceStatus] {
	var out execution.MaintenanceStatus
	_ = c.rpc.do(context.Background(), "arb_maintenanceStatus", []interface{}{}, &out)
	return containers.NewReadyPromise(&out, nil)
}

func (c *RethExecutionClient) Synced(ctx context.Context) bool {
	var out bool
	err := c.rpc.do(ctx, "arb_synced", []interface{}{}, &out)
	if err != nil {
		log.Warn("reth rpc synced failed", "err", err)
		return false
	}
	return out
}

func (c *RethExecutionClient) FullSyncProgressMap(ctx context.Context) map[string]interface{} {
	var out map[string]interface{}
	_ = c.rpc.do(ctx, "arb_fullSyncProgressMap", []interface{}{}, &out)
	if out == nil {
		out = map[string]interface{}{}
	}
	return out
}

func (c *RethExecutionClient) Pause()                                  {}
func (c *RethExecutionClient) Activate()                               {}
func (c *RethExecutionClient) ForwardTo(url string) error              { return nil }
func (c *RethExecutionClient) SequenceDelayedMessage(message *arbostypes.L1IncomingMessage, delayedSeqNum uint64) error {
	return fmt.Errorf("not implemented for reth rpc adapter yet")
}
func (c *RethExecutionClient) NextDelayedMessageNumber() (uint64, error) { return 0, fmt.Errorf("not implemented") }

func (c *RethExecutionClient) SetFinalityDataRPCOnly(ctx context.Context, safe, finalized, validated *arbutil.FinalityData) error {
	return nil
}
