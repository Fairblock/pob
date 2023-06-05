package auction

import (
	"github.com/cometbft/cometbft/libs/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/skip-mev/pob/blockbuster"
)

const (
	// LaneName defines the name of the top-of-block auction lane.
	LaneName = "top-of-block"
)

var (
	_ blockbuster.Lane = (*TOBLane)(nil)
	_ Factory          = (*TOBLane)(nil)
)

// TOBLane defines a top-of-block auction lane. The top of block auction lane
// hosts transactions that want to bid for inclusion at the top of the next block.
// The top of block auction lane stores bid transactions that are sorted by
// their bid price. The highest valid bid transaction is selected for inclusion in the
// next block. The bundled transactions of the selected bid transaction are also
// included in the next block.
type TOBLane struct {
	// Mempool defines the mempool for the lane.
	Mempool

	// LaneConfig defines the base lane configuration.
	cfg blockbuster.BaseLaneConfig

	// Factory defines the API/functionality which is responsible for determining
	// if a transaction is a bid transaction and how to extract relevant
	// information from the transaction (bid, timeout, bidder, etc.).
	Factory
}

// NewTOBLane returns a new TOB lane.
func NewTOBLane(
	cfg blockbuster.BaseLaneConfig,
	maxTx int,
	af Factory,
) *TOBLane {
	if err := cfg.ValidateBasic(); err != nil {
		panic(err)
	}

	return &TOBLane{
		Mempool: NewMempool(cfg.TxEncoder, maxTx, af),
		cfg:     cfg,
		Factory: af,
	}
}

// Match returns true if the transaction is a bid transaction. This is determined
// by the AuctionFactory.
func (l *TOBLane) Match(tx sdk.Tx) bool {
	bidInfo, err := l.GetAuctionBidInfo(tx)
	return bidInfo != nil && err == nil
}

// Name returns the name of the lane.
func (l *TOBLane) Name() string {
	return LaneName
}

// Logger returns the lane's logger.
func (l *TOBLane) Logger() log.Logger {
	return l.cfg.Logger
}

// SetAnteHandler sets the lane's configuration.
func (l *TOBLane) SetAnteHandler(anteHandler sdk.AnteHandler) {
	l.cfg.AnteHandler = anteHandler
}

// GetMaxBlockSpace returns the maximum block space for the lane.
func (l *TOBLane) GetMaxBlockSpace() sdk.Dec {
	return l.cfg.MaxBlockSpace
}
