package ccaas

import (
	"context"

	"github.com/vichcraft/ccas/pkg/types"
	pb "github.com/vichcraft/ccas/proto/ccaspb"
	"google.golang.org/grpc"
)

// GRPCCertifierClient implements Certifier via gRPC calls to a remote CertifierService.
type GRPCCertifierClient struct {
	client pb.CertifierServiceClient
}

// NewGRPCCertifierClient creates a Certifier backed by a gRPC connection.
func NewGRPCCertifierClient(conn *grpc.ClientConn) *GRPCCertifierClient {
	return &GRPCCertifierClient{client: pb.NewCertifierServiceClient(conn)}
}

func (c *GRPCCertifierClient) SyncFromStorage() {
	_, _ = c.client.SyncFromStorage(context.Background(), &pb.SyncFromStorageRequest{})
}

func (c *GRPCCertifierClient) SubmitCommit(req types.TxnRequest) types.TxnResult {
	pbReq := &pb.SubmitCommitRequest{
		TxnId:    req.TxnID,
		ReadSet:  make([]*pb.ReadEntry, len(req.ReadSet)),
		WriteSet: make([]*pb.WriteEntry, len(req.WriteSet)),
	}
	for i, r := range req.ReadSet {
		pbReq.ReadSet[i] = &pb.ReadEntry{Key: r.Key, VersionSeen: r.VersionSeen}
	}
	for i, w := range req.WriteSet {
		pbReq.WriteSet[i] = &pb.WriteEntry{Key: w.Key, Value: w.Value}
	}

	resp, err := c.client.SubmitCommit(context.Background(), pbReq)
	if err != nil {
		return types.TxnResult{
			TxnID:  req.TxnID,
			Reason: err.Error(),
		}
	}

	return types.TxnResult{
		TxnID:     resp.TxnId,
		Committed: resp.Committed,
		CommitSeq: resp.CommitSeq,
		Reason:    resp.Reason,
	}
}
