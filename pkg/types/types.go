package types

// ReadEntry is one element of a transaction's read set.
type ReadEntry struct {
	Key         string
	VersionSeen uint64
}

// WriteEntry is one element of a transaction's write set (pre-certification).
type WriteEntry struct {
	Key   string
	Value string
}

// TxnRequest is sent from the execution layer to CCaaS at commit time.
type TxnRequest struct {
	TxnID    string
	ReadSet  []ReadEntry
	WriteSet []WriteEntry
}

// CommitWrite is one write within a commit record (post-certification).
type CommitWrite struct {
	Key        string
	Value      string
	NewVersion uint64
}

// CommitRecord is emitted by CCaaS and applied by the storage layer.
type CommitRecord struct {
	CommitSeq uint64
	TxnID     string
	Writes    []CommitWrite
}

// TxnResult is the CCaaS decision returned to the execution layer.
type TxnResult struct {
	TxnID     string
	Committed bool
	CommitSeq uint64 // 0 if aborted
	Reason    string // empty on commit, explanation on abort
}

// ReadResult is returned by storage Get.
type ReadResult struct {
	Value   string
	Version uint64
	Found   bool
}
