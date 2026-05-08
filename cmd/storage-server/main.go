package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/vichcraft/ccas/pkg/storage"
	pb "github.com/vichcraft/ccas/proto/ccaspb"
	"google.golang.org/grpc"
)

func main() {
	port := flag.Int("port", 50051, "port to listen on")
	flag.Parse()

	store := storage.NewMemStore()
	const maxMsg = 256 * 1024 * 1024
	srv := grpc.NewServer(grpc.MaxRecvMsgSize(maxMsg), grpc.MaxSendMsgSize(maxMsg))
	pb.RegisterStorageServiceServer(srv, storage.NewGRPCStorageServer(store))

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutting down storage server...")
		srv.GracefulStop()
	}()

	log.Printf("Storage server listening on :%d", *port)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
