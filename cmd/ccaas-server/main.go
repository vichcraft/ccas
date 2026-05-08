package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vichcraft/ccas/pkg/ccaas"
	"github.com/vichcraft/ccas/pkg/storage"
	pb "github.com/vichcraft/ccas/proto/ccaspb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	port := flag.Int("port", 50052, "port to listen on")
	storageAddr := flag.String("storage-addr", "localhost:50051", "address of the storage server")
	epochMs := flag.Int("epoch-ms", 0, "epoch duration in ms (0 = immediate mode)")
	flag.Parse()

	// Connect to storage server. The 256MB ceiling lets DumpState carry full
	// state for ~1M-record workloads (each YCSB record is ~150 bytes).
	const maxMsg = 256 * 1024 * 1024
	storageConn, err := grpc.NewClient(*storageAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(maxMsg),
			grpc.MaxCallRecvMsgSize(maxMsg),
		),
	)
	if err != nil {
		log.Fatalf("failed to connect to storage at %s: %v", *storageAddr, err)
	}
	defer storageConn.Close()

	storeClient := storage.NewGRPCStoreClient(storageConn)

	// Create certifier.
	var certifier ccaas.Certifier
	var epochCertifier *ccaas.EpochCertifier

	if *epochMs > 0 {
		epochCertifier = ccaas.NewEpochCertifier(storeClient, time.Duration(*epochMs)*time.Millisecond)
		epochCertifier.Start()
		certifier = epochCertifier
		log.Printf("Using epoch-based certification (%dms)", *epochMs)
	} else {
		certifier = ccaas.NewImmediateCertifier(storeClient)
		log.Println("Using immediate certification")
	}

	srv := grpc.NewServer(grpc.MaxRecvMsgSize(maxMsg), grpc.MaxSendMsgSize(maxMsg))
	pb.RegisterCertifierServiceServer(srv, ccaas.NewGRPCCertifierServer(certifier))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutting down CCaaS server...")
		if epochCertifier != nil {
			epochCertifier.Stop()
		}
		srv.GracefulStop()
	}()

	log.Printf("CCaaS server listening on :%d, connected to storage at %s", *port, *storageAddr)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
