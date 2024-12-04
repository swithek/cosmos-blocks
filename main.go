package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"

	"github.com/schollz/progressbar/v3"
)

func main() {
	var (
		startHeight int64
		endHeight   int64
		nodeURL     string
		parallelism int
		outputFile  string
	)

	flag.Int64Var(&startHeight, "start-height", -1, "The height of the first fetched block")
	flag.Int64Var(&endHeight, "end-height", -1, "The height of the last fetched block")
	flag.StringVar(&nodeURL, "node-url", "", "The base URL of Cosmos RPC node")
	flag.IntVar(&parallelism, "parallelism", -1, "The number of concurrent workers to use when fetching block data")
	flag.StringVar(&outputFile, "output", "", "The path to the output file (e.g., /absolute/path/to/file)")
	flag.Parse()

	if startHeight < 0 {
		log.Fatal("--start-height must be 0 or greater")
	}

	if endHeight < 0 || endHeight < startHeight {
		log.Fatal("--end-height must be greater or equal to start height")
	}

	if nodeURL == "" {
		log.Fatal("--node-url must be provided")
	}

	if parallelism < 1 {
		log.Fatal("--parallelism must be 1 or greater")
	}

	if outputFile == "" {
		log.Fatal("--output must be provided")
	}

	var (
		wg                  sync.WaitGroup
		blockCh             = make(chan int64, parallelism)
		resultCh            = make(chan block, endHeight-startHeight+1)
		term                = make(chan os.Signal, 1)
		rootCtx, rootCancel = context.WithCancel(context.Background())
		cl                  = newClient(nodeURL, maxRetries)
		bar                 = progressbar.Default(endHeight - startHeight + 1)
	)

	signal.Notify(term, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-term
		rootCancel()
	}()

	networkID, err := cl.fetchNetworkID(rootCtx)
	if err != nil {
		log.Fatal("There was an error fetching network status info:", err)
	}

	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			werr := startWorker(rootCtx, cl, networkID, blockCh, resultCh)

			// an error can happen only in the most critical and
			// unrecoverable situations (e.g., with a wrong node URL)
			if werr != nil {
				log.Fatal("There was an error fetching block info:", err)
			}
		}()
	}

	for currentHeight := startHeight; currentHeight <= endHeight; currentHeight++ {
		blockCh <- currentHeight

		if err := bar.Add(1); err != nil {
			log.Fatal("There was an error updating progress bar:", err)
		}
	}

	log.Println("Blocks downloaded, preparing to save them to a file")

	close(blockCh)
	wg.Wait()
	close(resultCh)

	var blocks []block
	for b := range resultCh {
		blocks = append(blocks, b)
	}

	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Height < blocks[j].Height
	})

	if err = saveToJSONFile(outputFile, blocks); err != nil {
		log.Fatal("There was an error saving block data to a JSON file:", err)
	}

	log.Println("Blocks saved to a file")
}

// startWorker starts a new worker that listens for blocks to fetch
// on the block channel.
// Note that this function blocks until the block channel is closed.
func startWorker(ctx context.Context, cl *client, networkID string, blockCh <-chan int64, resultCh chan<- block) error {
	for height := range blockCh {
		res, err := cl.fetchBlock(ctx, networkID, height)
		if err != nil {
			return err
		}

		resultCh <- res
	}

	return nil
}

// saveToJSONFile saves all blocks to a single JSON file.
func saveToJSONFile(filename string, data []block) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}

	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "\t")

	return enc.Encode(data)
}
