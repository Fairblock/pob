package blockbuster

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

func GetTxHashStr(txEncoder sdk.TxEncoder, tx sdk.Tx) (string, error) {
	txBz, err := txEncoder(tx)
	if err != nil {
		return "", fmt.Errorf("failed to encode transaction: %w", err)
	}

	txHash := sha256.Sum256(txBz)
	txHashStr := hex.EncodeToString(txHash[:])

	return txHashStr, nil
}

func RemoveTxsFromMempool(txs map[sdk.Tx]struct{}, mempool sdkmempool.Mempool) error {
	for tx := range txs {
		if err := mempool.Remove(tx); err != nil {
			return err
		}
	}

	return nil
}
