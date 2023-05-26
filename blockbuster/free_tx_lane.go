package blockbuster

import (
	"context"

	"github.com/cometbft/cometbft/libs/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/skip-mev/pob/mempool"
)

const (
	// LaneNameFreeTx defines the name of the free transaction lane.
	LaneNameFreeTx = "free-tx"
)

var _ Lane = (*FreeTxLane)(nil)

type FreeTxLane struct {
	logger      log.Logger
	index       sdkmempool.Mempool
	af          mempool.AuctionFactory
	txEncoder   sdk.TxEncoder
	txDecoder   sdk.TxDecoder
	anteHandler sdk.AnteHandler

	// txIndex is a map of all transactions in the mempool. It is used
	// to quickly check if a transaction is already in the mempool.
	txIndex map[string]struct{}
}

func NewFreeTxLane(logger log.Logger, txDecoder sdk.TxDecoder, txEncoder sdk.TxEncoder, maxTx int, af mempool.AuctionFactory, anteHandler sdk.AnteHandler) *FreeTxLane {
	return &FreeTxLane{
		logger: logger,
		index: mempool.NewPriorityMempool(
			mempool.PriorityNonceMempoolConfig[int64]{
				TxPriority: mempool.NewDefaultTxPriority(),
				MaxTx:      maxTx,
			},
		),
		af:          af,
		txEncoder:   txEncoder,
		txDecoder:   txDecoder,
		anteHandler: anteHandler,
		txIndex:     make(map[string]struct{}),
	}
}

func (l *FreeTxLane) Name() string {
	panic("not implemented")
}

func (l *FreeTxLane) Match(tx sdk.Tx) bool {
	panic("not implemented")
}

func (l *FreeTxLane) VerifyTx(ctx sdk.Context, tx sdk.Tx) error {
	panic("not implemented")
}

func (l *FreeTxLane) Contains(tx sdk.Tx) (bool, error) {
	panic("not implemented")
}

func (l *FreeTxLane) PrepareLane(ctx sdk.Context, maxTxBytes int64, selectedTxs map[string][]byte) ([][]byte, error) {
	panic("not implemented")
}

func (l *FreeTxLane) ProcessLane(ctx sdk.Context, proposalTxs [][]byte) error {
	panic("not implemented")
}

func (l *FreeTxLane) Insert(context.Context, sdk.Tx) error {
	panic("not implemented")
}

func (l *FreeTxLane) Select(context.Context, [][]byte) sdkmempool.Iterator {
	panic("not implemented")
}

func (l *FreeTxLane) CountTx() int {
	panic("not implemented")
}

func (l *FreeTxLane) Remove(sdk.Tx) error {
	panic("not implemented")
}
