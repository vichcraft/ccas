package execution

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/vichcraft/ccas/pkg/ccaas"
	"github.com/vichcraft/ccas/pkg/storage"
	"github.com/vichcraft/ccas/pkg/types"
)

var (
	ErrTxnAborted   = errors.New("transaction is aborted")
	ErrTxnCommitted = errors.New("transaction is already committed")
)

// Txn represents an in-flight transaction.
type Txn struct {
	ID           string
	ReadSet      []types.ReadEntry
	WriteSet     []types.WriteEntry
	writeBuffer  map[string]string
	readVersions map[string]uint64
	aborted      bool
	committed    bool
}

// Engine is the execution layer runtime.
type Engine struct {
	store     storage.Store
	certifier ccaas.Certifier
	idCounter uint64
}

// NewEngine creates an Engine wired to the given store and certifier.
func NewEngine(store storage.Store, certifier ccaas.Certifier) *Engine {
	return &Engine{
		store:     store,
		certifier: certifier,
	}
}

// Begin starts a new transaction and returns it.
func (e *Engine) Begin() *Txn {
	id := atomic.AddUint64(&e.idCounter, 1)
	return &Txn{
		ID:           fmt.Sprintf("txn-%d", id),
		writeBuffer:  make(map[string]string),
		readVersions: make(map[string]uint64),
	}
}

// Read retrieves a key's value within a transaction.
// It returns the write buffer value if the txn previously wrote to the key (read-your-own-writes).
// Otherwise it reads from storage and records the version in the read set.
func (e *Engine) Read(txn *Txn, key string) (string, error) {
	if err := checkActive(txn); err != nil {
		return "", err
	}

	// Read-your-own-writes: return buffered value, no read-set entry.
	if val, ok := txn.writeBuffer[key]; ok {
		return val, nil
	}

	result := e.store.Get(key)

	// Only add to read set on the first storage read of this key.
	if _, seen := txn.readVersions[key]; !seen {
		version := result.Version
		if !result.Found {
			version = 0
		}
		txn.ReadSet = append(txn.ReadSet, types.ReadEntry{Key: key, VersionSeen: version})
		txn.readVersions[key] = version
	}

	if !result.Found {
		return "", nil
	}
	return result.Value, nil
}

// Write buffers a write locally. Storage is not contacted.
func (e *Engine) Write(txn *Txn, key string, value string) error {
	if err := checkActive(txn); err != nil {
		return err
	}

	txn.WriteSet = append(txn.WriteSet, types.WriteEntry{Key: key, Value: value})
	txn.writeBuffer[key] = value
	return nil
}

// Commit submits the transaction's read and write sets to the certifier.
func (e *Engine) Commit(txn *Txn) (types.TxnResult, error) {
	if err := checkActive(txn); err != nil {
		return types.TxnResult{}, err
	}

	req := types.TxnRequest{
		TxnID:    txn.ID,
		ReadSet:  txn.ReadSet,
		WriteSet: txn.WriteSet,
	}

	result := e.certifier.SubmitCommit(req)

	if result.Committed {
		txn.committed = true
	} else {
		txn.aborted = true
	}

	return result, nil
}

// Abort marks the transaction as aborted and discards buffered writes.
func (e *Engine) Abort(txn *Txn) {
	txn.aborted = true
	txn.writeBuffer = nil
}

func checkActive(txn *Txn) error {
	if txn.aborted {
		return ErrTxnAborted
	}
	if txn.committed {
		return ErrTxnCommitted
	}
	return nil
}
