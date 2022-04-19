package api

import (
	"context"
	"encoding/json"
	"mergemock/rpc"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethRpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/sirupsen/logrus"
)

type ExecutionPayloadHeaderV1 struct {
	ParentHash       common.Hash     `json:"parentHash"`
	FeeRecipient     common.Address  `json:"feeRecipient"`
	StateRoot        Bytes32         `json:"stateRoot"`
	ReceiptsRoot     Bytes32         `json:"receiptsRoot"`
	LogsBloom        Bytes256        `json:"logsBloom"`
	PrevRandao       Bytes32         `json:"prevRandao"`
	BlockNumber      Uint64Quantity  `json:"blockNumber"`
	GasLimit         Uint64Quantity  `json:"gasLimit"`
	GasUsed          Uint64Quantity  `json:"gasUsed"`
	Timestamp        Uint64Quantity  `json:"timestamp"`
	ExtraData        BytesMax32      `json:"extraData"`
	BaseFeePerGas    Uint256Quantity `json:"baseFeePerGas"`
	BlockHash        common.Hash     `json:"blockHash"`
	TransactionsRoot Bytes32         `json:"transactionsRoot"`
	FeeRecipientDiff Uint256Quantity `json:"feeRecipientDiff"`
}

type GetHeaderResponseMessage struct {
	Header ExecutionPayloadHeaderV1 `json:"header"`
	Value  Uint256Quantity          `json:"value"`
}

type GetHeaderResponse struct {
	Message   GetHeaderResponseMessage `json:"message"`
	PublicKey BytesMax32               `json:"publicKey"`
	Signature BytesMax32               `json:"signature"`
}

// See https://github.com/flashbots/mev-boost#signedblindedbeaconblock
type SignedBlindedBeaconBlock struct {
	Message   *BlindedBeaconBlock `json:"message"`
	Signature string              `json:"signature"`
}

// See https://github.com/flashbots/mev-boost#blindedbeaconblock
type BlindedBeaconBlock struct {
	Body BlindedBeaconBlockBody `json:"body"`
}

type BlindedBeaconBlockBody struct {
	ExecutionPayload ExecutionPayloadHeaderV1 `json:"execution_payload_header"`
}

type SignedBuilderReceipt struct {
	Message   *BuilderReceipt `json:"message"`
	Signature string          `json:"signature"`
}

type BuilderReceipt struct {
	PayloadHeader    ExecutionPayloadHeaderV1 `json:"execution_payload_header"`
	FeeRecipientDiff Uint256Quantity          `json:"feeRecipientDiff"`
}

func BuilderGetHeader(ctx context.Context, cl *rpc.Client, log logrus.Ext1FieldLogger, blockHash common.Hash) (*ExecutionPayloadHeaderV1, error) {
	e := log.WithField("blockHash", blockHash)
	e.Debug("getting payload")
	var result GetHeaderResponse

	err := cl.CallContext(ctx, &result, "builder_getHeaderV1", blockHash)
	if err != nil {
		e = e.WithError(err)
		if rpcErr, ok := err.(gethRpc.Error); ok {
			code := ErrorCode(rpcErr.ErrorCode())
			if code != UnavailablePayload {
				e.WithField("code", code).Warn("unexpected error code in get-payload header response")
			} else {
				e.Warn("unavailable payload in get-payload header request")
			}
		} else {
			e.Error("failed to get payload header")
		}
		return nil, err
	}
	e.Debug("Received payload")
	return &result.Message.Header, nil
}

func BuilderGetPayload(ctx context.Context, cl *rpc.Client, log logrus.Ext1FieldLogger, header *ExecutionPayloadHeaderV1) (*ExecutionPayloadV1, error) {
	e := log.WithField("block_hash", header.BlockHash)
	e.Debug("sending payload for execution")
	var result ExecutionPayloadV1

	beaconBlock := BlindedBeaconBlock{
		Body: BlindedBeaconBlockBody{ExecutionPayload: *header},
	}

	// TODO: SSZ-encode SignedBlindedBeaconBlock
	encoded_block, err := json.Marshal(SignedBlindedBeaconBlock{Message: &beaconBlock})
	if err != nil {
		e.WithError(err).Warn("unable to marshal beacon block")
		return nil, err
	}

	err = cl.CallContext(ctx, &result, "builder_getPayloadV1", string(encoded_block))
	if err != nil {
		e = e.WithError(err)
		if rpcErr, ok := err.(gethRpc.Error); ok {
			code := ErrorCode(rpcErr.ErrorCode())
			if code != UnavailablePayload {
				e.WithField("code", code).Warn("unexpected error code in propose-payload response")
			} else {
				e.Warn("unavailable payload in propose-payload request")
			}
		} else {
			e.Error("failed to propose payload")
		}
		return nil, err
	}
	e.Debug("Received proposed payload")
	return &result, nil
}

func PayloadToPayloadHeader(p *ExecutionPayloadV1) (*ExecutionPayloadHeaderV1, error) {
	txs, err := decodeTransactions(p.Transactions)
	if err != nil {
		return nil, err
	}
	return &ExecutionPayloadHeaderV1{
		ParentHash:       p.ParentHash,
		FeeRecipient:     p.FeeRecipient,
		StateRoot:        p.StateRoot,
		ReceiptsRoot:     p.ReceiptsRoot,
		LogsBloom:        p.LogsBloom,
		PrevRandao:       p.PrevRandao,
		BlockNumber:      p.BlockNumber,
		GasLimit:         p.GasLimit,
		GasUsed:          p.GasUsed,
		Timestamp:        p.Timestamp,
		ExtraData:        p.ExtraData,
		BaseFeePerGas:    p.BaseFeePerGas,
		BlockHash:        p.BlockHash,
		TransactionsRoot: Bytes32(types.DeriveSha(types.Transactions(txs), trie.NewStackTrie(nil))),
	}, nil
}
