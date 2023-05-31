package blockbuster

import (
	"github.com/cometbft/cometbft/libs/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

type (
	// LaneConfig defines the configuration for a lane.
	LaneConfig struct {
		Logger        log.Logger
		TxEncoder     sdk.TxEncoder
		TxDecoder     sdk.TxDecoder
		AnteHandler   sdk.AnteHandler
		MaxBlockSpace sdk.Dec

		// Key defines the name of the lane.
		Key string
	}

	// Lane defines an interface used for block construction
	Lane interface {
		sdkmempool.Mempool

		// Name returns the name of the lane.
		Name() string

		// Match determines if a transaction belongs to this lane.
		Match(tx sdk.Tx) bool

		// VerifyTx verifies the transaction belonging to this lane.
		VerifyTx(ctx sdk.Context, tx sdk.Tx) error

		// Contains returns true if the mempool contains the given transaction.
		Contains(tx sdk.Tx) (bool, error)

		// PrepareLane which builds a portion of the block. Inputs include the max
		// number of bytes that can be included in the block and the selected transactions
		// thus from from previous lane(s) as mapping from their HEX-encoded hash to
		// the raw transaction.
		PrepareLane(ctx sdk.Context, proposal Proposal, next PrepareLanesHandler) Proposal

		// ProcessLane which verifies the lane's portion of a proposed block. Returns an error
		// if the lane's portion of the block is invalid. Also returns the index of the next
		// transcation that does not belong to this. Lanes are verified in a greedy fashion.
		ProcessLane(ctx sdk.Context, proposalTxs [][]byte, next ProcessLanesHandler) (sdk.Context, error)
	}
)

func NewLaneConfig(logger log.Logger, txEncoder sdk.TxEncoder, txDecoder sdk.TxDecoder,
	anteHandler sdk.AnteHandler, key string, maxBlockSpace sdk.Dec) *LaneConfig {
	return &LaneConfig{
		Logger:        logger,
		TxEncoder:     txEncoder,
		TxDecoder:     txDecoder,
		AnteHandler:   anteHandler,
		Key:           key,
		MaxBlockSpace: maxBlockSpace,
	}
}

func (c LaneConfig) Name() string {
	return c.Key
}
