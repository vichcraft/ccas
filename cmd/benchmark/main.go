package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/vichcraft/ccas/pkg/benchmark"
	"github.com/vichcraft/ccas/pkg/ccaas"
	"github.com/vichcraft/ccas/pkg/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	config := benchmark.DefaultConfig()

	mode := flag.String("mode", "local", "run mode: local or remote")
	storageAddr := flag.String("storage-addr", "localhost:50051", "storage server address (remote mode)")
	ccaasAddr := flag.String("ccaas-addr", "localhost:50052", "CCaaS server address (remote mode)")

	flag.IntVar(&config.NumClients, "clients", config.NumClients, "number of concurrent clients")
	flag.IntVar(&config.NumKeys, "keys", config.NumKeys, "key space size")
	flag.IntVar(&config.ReadsPerTxn, "reads-per-txn", config.ReadsPerTxn, "reads per transaction")
	flag.IntVar(&config.WritesPerTxn, "writes-per-txn", config.WritesPerTxn, "writes per transaction")
	flag.IntVar(&config.TotalTxnsPerClient, "txns-per-client", config.TotalTxnsPerClient, "transactions per client")
	flag.StringVar(&config.ContentionMode, "contention", config.ContentionMode, "contention mode: uniform or hotspot")
	flag.Float64Var(&config.HotKeyFraction, "hot-key-fraction", config.HotKeyFraction, "fraction of keys that are hot")
	flag.Float64Var(&config.HotKeyAccessProb, "hot-key-access-prob", config.HotKeyAccessProb, "probability of accessing a hot key")
	flag.IntVar(&config.EpochDurationMs, "epoch-ms", config.EpochDurationMs, "epoch duration in ms (0 = immediate mode)")
	flag.BoolVar(&config.RetryOnAbort, "retry", config.RetryOnAbort, "retry aborted transactions")
	flag.Parse()

	certMode := "immediate"
	if config.EpochDurationMs > 0 {
		certMode = fmt.Sprintf("epoch (%dms)", config.EpochDurationMs)
	}

	fmt.Println("CCaaS Benchmark")
	fmt.Println("===============")
	fmt.Printf("Mode:            %s (%s)\n", *mode, certMode)
	fmt.Printf("Clients:         %d\n", config.NumClients)
	fmt.Printf("Keys:            %d\n", config.NumKeys)
	fmt.Printf("Reads/Txn:       %d\n", config.ReadsPerTxn)
	fmt.Printf("Writes/Txn:      %d\n", config.WritesPerTxn)
	fmt.Printf("Txns/Client:     %d\n", config.TotalTxnsPerClient)
	fmt.Printf("Contention:      %s\n", config.ContentionMode)
	fmt.Printf("Retry on abort:  %v\n", config.RetryOnAbort)

	var rc benchmark.RunConfig
	rc.Config = config

	if *mode == "remote" {
		fmt.Printf("Storage:         %s\n", *storageAddr)
		fmt.Printf("CCaaS:           %s\n", *ccaasAddr)

		storageConn, err := grpc.NewClient(*storageAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("failed to connect to storage: %v", err)
		}
		defer storageConn.Close()

		ccaasConn, err := grpc.NewClient(*ccaasAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("failed to connect to ccaas: %v", err)
		}
		defer ccaasConn.Close()

		storeClient := storage.NewGRPCStoreClient(storageConn)
		certClient := ccaas.NewGRPCCertifierClient(ccaasConn)

		// Load initial data on the remote storage server.
		data := make(map[string]string, config.NumKeys)
		for i := range config.NumKeys {
			data[fmt.Sprintf("key-%d", i)] = "0"
		}
		storeClient.LoadInitialData(data)

		// Re-sync CCaaS version map after data is loaded.
		certClient.SyncFromStorage()

		rc.Store = storeClient
		rc.Certifier = certClient
	}

	fmt.Println()
	fmt.Println("Running...")

	report := benchmark.RunWith(rc)

	fmt.Println()
	fmt.Println("Results")
	fmt.Println("-------")
	fmt.Printf("Committed:       %d\n", report.TotalCommitted)
	fmt.Printf("Aborted:         %d\n", report.TotalAborted)
	fmt.Printf("Abort Rate:      %.2f%%\n", report.AbortRate*100)
	fmt.Printf("Throughput:      %.0f txn/s\n", report.Throughput)
	fmt.Printf("Avg Latency:     %s\n", report.AvgLatency)
	fmt.Printf("P95 Latency:     %s\n", report.P95Latency)
	fmt.Printf("Avg Epoch Size:  %.1f\n", report.AvgEpochSize)
	fmt.Printf("Conflicts:       %d\n", report.ConflictCount)
	fmt.Printf("Elapsed:         %s\n", report.ElapsedTime.Round(time.Millisecond))
}
