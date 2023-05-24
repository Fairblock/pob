package blockbuster

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/cometbft/cometbft/libs/log"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/skip-mev/pob/mempool"
)

const (
	// LaneNameTOB defines the name of the top-of-block auction lane.
	LaneNameTOB = "tob"
)

var _ Lane = (*TOBLane)(nil)

type TOBLane struct {
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

func NewTOBLane(logger log.Logger, txDecoder sdk.TxDecoder, txEncoder sdk.TxEncoder, maxTx int, af mempool.AuctionFactory, anteHandler sdk.AnteHandler) *TOBLane {
	return &TOBLane{
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

func (l *TOBLane) Name() string {
	return LaneNameTOB
}

func (l *TOBLane) Match(tx sdk.Tx) bool {
	bidInfo, err := l.af.GetAuctionBidInfo(tx)
	return bidInfo != nil && err == nil
}

func (l *TOBLane) Contains(tx sdk.Tx) (bool, error) {
	txHashStr, err := l.getTxHashStr(tx)
	if err != nil {
		return false, fmt.Errorf("failed to get tx hash string: %w", err)
	}

	_, ok := l.txIndex[txHashStr]
	return ok, nil
}

func (l *TOBLane) VerifyTx(ctx sdk.Context, bidTx sdk.Tx) error {
	bidInfo, err := l.af.GetAuctionBidInfo(bidTx)
	if err != nil {
		return fmt.Errorf("failed to get auction bid info: %w", err)
	}

	// verify the top-level bid transaction
	ctx, err = l.verifyTx(ctx, bidTx)
	if err != nil {
		return fmt.Errorf("invalid bid tx; failed to execute ante handler: %w", err)
	}

	// verify all of the bundled transactions
	for _, tx := range bidInfo.Transactions {
		bundledTx, err := l.af.WrapBundleTransaction(tx)
		if err != nil {
			return fmt.Errorf("invalid bid tx; failed to decode bundled tx: %w", err)
		}

		// bid txs cannot be included in bundled txs
		bidInfo, _ := l.af.GetAuctionBidInfo(bundledTx)
		if bidInfo != nil {
			return fmt.Errorf("invalid bid tx; bundled tx cannot be a bid tx")
		}

		if ctx, err = l.verifyTx(ctx, bundledTx); err != nil {
			return fmt.Errorf("invalid bid tx; failed to execute bundled transaction: %w", err)
		}
	}

	return nil
}

// PrepareLane which builds a portion of the block. Inputs a cache of transactions
// that have already been included by a previous lane.
func (l *TOBLane) PrepareLane(ctx sdk.Context, maxTxBytes int64, selectedTxs map[string][]byte) ([][]byte, error) {
	var (
		tmpSelectedTxs [][]byte
		totalTxBytes   int64
	)

	// compute the total size of the transactions selected thus far
	for _, tx := range selectedTxs {
		totalTxBytes += int64(len(tx))
	}

	bidTxIterator := l.index.Select(ctx, nil)
	txsToRemove := make(map[sdk.Tx]struct{}, 0)

	// Attempt to select the highest bid transaction that is valid and whose
	// bundled transactions are valid.
selectBidTxLoop:
	for ; bidTxIterator != nil; bidTxIterator = bidTxIterator.Next() {
		cacheCtx, write := ctx.CacheContext()
		tmpBidTx := bidTxIterator.Tx()

		// if the transaction is already in the (partial) block proposal, we skip it
		txHash, err := l.getTxHashStr(tmpBidTx)
		if err != nil {
			return nil, fmt.Errorf("failed to get bid tx hash: %w", err)
		}
		if _, ok := selectedTxs[txHash]; ok {
			continue selectBidTxLoop
		}

		bidTxBz, err := l.prepareProposalVerifyTx(cacheCtx, tmpBidTx)
		if err != nil {
			txsToRemove[tmpBidTx] = struct{}{}
			continue selectBidTxLoop
		}

		bidTxSize := int64(len(bidTxBz))
		if bidTxSize <= maxTxBytes {
			bidInfo, err := l.af.GetAuctionBidInfo(tmpBidTx)
			if bidInfo == nil || err != nil {
				// Some transactions in the bundle may be malformed or invalid, so we
				// remove the bid transaction and try the next top bid.
				txsToRemove[tmpBidTx] = struct{}{}
				continue selectBidTxLoop
			}

			// store the bytes of each ref tx as sdk.Tx bytes in order to build a valid proposal
			bundledTransactions := bidInfo.Transactions
			sdkTxBytes := make([][]byte, len(bundledTransactions))

			// Ensure that the bundled transactions are valid
			for index, rawRefTx := range bundledTransactions {
				refTx, err := l.af.WrapBundleTransaction(rawRefTx)
				if err != nil {
					// Malformed bundled transaction, so we remove the bid transaction
					// and try the next top bid.
					txsToRemove[tmpBidTx] = struct{}{}
					continue selectBidTxLoop
				}

				txBz, err := l.prepareProposalVerifyTx(cacheCtx, refTx)
				if err != nil {
					// Invalid bundled transaction, so we remove the bid transaction
					// and try the next top bid.
					txsToRemove[tmpBidTx] = struct{}{}
					continue selectBidTxLoop
				}

				sdkTxBytes[index] = txBz
			}

			// At this point, both the bid transaction itself and all the bundled
			// transactions are valid. So we select the bid transaction along with
			// all the bundled transactions. We also mark these transactions as seen and
			// update the total size selected thus far.
			totalTxBytes += bidTxSize
			tmpSelectedTxs = append(tmpSelectedTxs, bidTxBz)
			tmpSelectedTxs = append(tmpSelectedTxs, sdkTxBytes...)

			// Write the cache context to the original context when we know we have a
			// valid top of block bundle.
			write()

			break selectBidTxLoop
		}

		txsToRemove[tmpBidTx] = struct{}{}
		l.logger.Info(
			"failed to select auction bid tx; tx size is too large",
			"tx_size", bidTxSize,
			"max_size", maxTxBytes,
		)
	}

	// Remove all invalid transactions from the mempool.
	for tx := range txsToRemove {
		if err := l.Remove(tx); err != nil {
			return nil, err
		}
	}

	return tmpSelectedTxs, nil
}

// ProcessLane which verifies the lane's portion of a proposed block.
func (l *TOBLane) ProcessLane(ctx sdk.Context, txs [][]byte) error {
	panic("not implemented")
}

func (l *TOBLane) Insert(goCtx context.Context, tx sdk.Tx) error {
	txHashStr, err := l.getTxHashStr(tx)
	if err != nil {
		return err
	}

	if err := l.index.Insert(goCtx, tx); err != nil {
		return fmt.Errorf("failed to insert tx into auction index: %w", err)
	}

	l.txIndex[txHashStr] = struct{}{}
	return nil
}

func (l *TOBLane) Select(goCtx context.Context, txs [][]byte) sdkmempool.Iterator {
	return l.index.Select(goCtx, txs)
}

func (l *TOBLane) CountTx() int {
	return l.index.CountTx()
}

func (l *TOBLane) Remove(tx sdk.Tx) error {
	txHashStr, err := l.getTxHashStr(tx)
	if err != nil {
		return fmt.Errorf("failed to get tx hash string: %w", err)
	}

	if err := l.index.Remove(tx); err != nil && !errors.Is(err, sdkmempool.ErrTxNotFound) {
		return fmt.Errorf("failed to remove invalid transaction from the mempool: %w", err)
	}

	delete(l.txIndex, txHashStr)
	return nil
}

func (l *TOBLane) prepareProposalVerifyTx(ctx sdk.Context, tx sdk.Tx) ([]byte, error) {
	txBz, err := l.txEncoder(tx)
	if err != nil {
		return nil, err
	}

	if _, err := l.verifyTx(ctx, tx); err != nil {
		return nil, err
	}

	return txBz, nil
}

func (l *TOBLane) processProposalVerifyTx(ctx sdk.Context, txBz []byte) (sdk.Tx, error) {
	tx, err := l.txDecoder(txBz)
	if err != nil {
		return nil, err
	}

	if _, err := l.verifyTx(ctx, tx); err != nil {
		return nil, err
	}

	return tx, nil
}

func (l *TOBLane) verifyTx(ctx sdk.Context, tx sdk.Tx) (sdk.Context, error) {
	if l.anteHandler != nil {
		newCtx, err := l.anteHandler(ctx, tx, false)
		return newCtx, err
	}

	return ctx, nil
}

// getTxHashStr returns the transaction hash string for a given transaction.
func (l *TOBLane) getTxHashStr(tx sdk.Tx) (string, error) {
	txBz, err := l.txEncoder(tx)
	if err != nil {
		return "", fmt.Errorf("failed to encode transaction: %w", err)
	}

	txHash := sha256.Sum256(txBz)
	txHashStr := hex.EncodeToString(txHash[:])

	return txHashStr, nil
}
