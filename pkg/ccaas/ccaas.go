package ccaas

import "github.com/vichcraft/ccas/pkg/types"

// Certifier is the concurrency-control contract exposed to the execution layer.
type Certifier interface {
	SubmitCommit(req types.TxnRequest) types.TxnResult
}
