package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/vichcraft/ccas/pkg/ccaas"
	"github.com/vichcraft/ccas/pkg/execution"
	"github.com/vichcraft/ccas/pkg/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	numAccounts    = 10
	initialBalance = 1000
	numClients     = 8
	txnsPerClient  = 100
	maxTransfer    = 100
)

func main() {
	mode := flag.String("mode", "local", "run mode: local or remote")
	storageAddr := flag.String("storage-addr", "localhost:50051", "storage server address (remote mode)")
	ccaasAddr := flag.String("ccaas-addr", "localhost:50052", "CCaaS server address (remote mode)")
	flag.Parse()

	fmt.Println("Bank Transfer Demo (CCaaS Prototype)")
	fmt.Println("=====================================")
	fmt.Printf("Accounts: %d, Initial balance: %d each\n", numAccounts, initialBalance)
	fmt.Printf("Clients: %d, Transfers per client: %d\n", numClients, txnsPerClient)
	fmt.Printf("Mode: %s\n", *mode)
	fmt.Println()

	if *mode == "remote" {
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

		fmt.Printf("Running with REMOTE certification (storage=%s, ccaas=%s)...\n", *storageAddr, *ccaasAddr)

		// runDemoWith loads data then syncs the certifier.
		runDemoWith(storeClient, certClient)
	} else {
		runDemo("IMMEDIATE", 0)
		fmt.Println()
		runDemo("EPOCH", 10)
	}
}

func runDemoWith(store storage.Store, certifier ccaas.Certifier) {
	initData := make(map[string]string, numAccounts)
	for i := range numAccounts {
		initData[fmt.Sprintf("acct-%d", i)] = strconv.Itoa(initialBalance)
	}
	store.LoadInitialData(initData)
	certifier.SyncFromStorage()

	engine := execution.NewEngine(store, certifier)
	committed, aborted := runTransfers(engine)
	printResults(store, committed, aborted)
}

func runDemo(label string, epochMs int) {
	ms := storage.NewMemStore()
	data := make(map[string]string, numAccounts)
	for i := range numAccounts {
		data[fmt.Sprintf("acct-%d", i)] = strconv.Itoa(initialBalance)
	}
	ms.LoadInitialData(data)

	var certifier ccaas.Certifier
	var epochCertifier *ccaas.EpochCertifier

	if epochMs > 0 {
		epochCertifier = ccaas.NewEpochCertifier(ms, time.Duration(epochMs)*time.Millisecond)
		epochCertifier.Start()
		certifier = epochCertifier
		fmt.Printf("Running with %s certification (%dms)...\n", label, epochMs)
	} else {
		certifier = ccaas.NewImmediateCertifier(ms)
		fmt.Printf("Running with %s certification...\n", label)
	}

	engine := execution.NewEngine(ms, certifier)
	committed, aborted := runTransfers(engine)

	if epochCertifier != nil {
		epochCertifier.Stop()
	}

	printResults(ms, committed, aborted)
}

func runTransfers(engine *execution.Engine) (committed, aborted int) {
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := range numClients {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(clientID) + time.Now().UnixNano()))

			for j := 0; j < txnsPerClient; j++ {
				src := rng.Intn(numAccounts)
				dst := rng.Intn(numAccounts)
				for dst == src {
					dst = rng.Intn(numAccounts)
				}
				amount := rng.Intn(maxTransfer) + 1

				srcKey := fmt.Sprintf("acct-%d", src)
				dstKey := fmt.Sprintf("acct-%d", dst)

				txn := engine.Begin()
				srcVal, _ := engine.Read(txn, srcKey)
				dstVal, _ := engine.Read(txn, dstKey)

				srcBal, _ := strconv.Atoi(srcVal)
				dstBal, _ := strconv.Atoi(dstVal)

				if srcBal >= amount {
					_ = engine.Write(txn, srcKey, strconv.Itoa(srcBal-amount))
					_ = engine.Write(txn, dstKey, strconv.Itoa(dstBal+amount))
					result, _ := engine.Commit(txn)

					mu.Lock()
					if result.Committed {
						committed++
					} else {
						aborted++
					}
					mu.Unlock()
				} else {
					engine.Abort(txn)
					mu.Lock()
					aborted++
					mu.Unlock()
				}
			}
		}(i)
	}
	wg.Wait()
	return
}

func printResults(store storage.Store, committed, aborted int) {
	total := committed + aborted
	abortRate := 0.0
	if total > 0 {
		abortRate = float64(aborted) / float64(total) * 100
	}

	balances := make([]int, numAccounts)
	sum := 0
	for i := range numAccounts {
		key := fmt.Sprintf("acct-%d", i)
		r := store.Get(key)
		bal, _ := strconv.Atoi(r.Value)
		balances[i] = bal
		sum += bal
	}

	expected := numAccounts * initialBalance

	fmt.Printf("  Committed: %d  Aborted: %d  Abort Rate: %.1f%%\n", committed, aborted, abortRate)
	fmt.Printf("  Final balances: %v\n", balances)
	if sum == expected {
		fmt.Printf("  Balance sum: %d (expected: %d) -- PASS\n", sum, expected)
	} else {
		fmt.Printf("  Balance sum: %d (expected: %d) -- FAIL\n", sum, expected)
	}
}
