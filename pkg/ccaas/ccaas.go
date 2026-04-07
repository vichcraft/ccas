package ccaas

import "github.com/vichcraft/ccas/pkg/types"

// Certifier is the concurrency-control contract exposed to the execution layer.
type Certifier interface {
	SubmitCommit(req types.TxnRequest) types.TxnResult
	// SyncFromStorage re-initializes the certifier's version map from storage.
	// Must be called after data is loaded into storage when the certifier was
	// created before data existed (e.g., in a multi-process setup).
	SyncFromStorage()
}
