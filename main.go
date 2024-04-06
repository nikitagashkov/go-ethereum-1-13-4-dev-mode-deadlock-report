package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"log"
	"sync"
	"time"
)

func getCoinbase(c *rpc.Client) common.Address {
	var coinbase common.Address

	if err := c.Call(&coinbase, "eth_coinbase"); err != nil {
		log.Fatalln("error obtaining coinbase", err)
	} else {
		log.Printf("coinbase=%v", coinbase)
	}

	return coinbase
}

func getTransactionCount(c *rpc.Client, address common.Address) hexutil.Uint {
	var nonce hexutil.Uint

	if err := c.Call(&nonce, "eth_getTransactionCount", address, "latest"); err != nil {
		log.Fatalln("error obtaining coinbase's initial nonce", err)
	} else {
		log.Printf("nonce=%v", nonce)
	}

	return nonce
}

func sendTransactionToSelf(c *rpc.Client, address common.Address, nonce hexutil.Uint) (common.Hash, error) {
	txObject := map[string]any{"from": address, "to": address, "nonce": nonce}

	var txHash common.Hash
	if err := c.Call(&txHash, "eth_sendTransaction", txObject); err != nil {
		return common.Hash{}, err
	}

	return txHash, nil
}

func waitTransactionReceipt(c *rpc.Client, txHash common.Hash) (*types.Receipt, error) {
	for i := 0; i < 10; i++ {
		var txReceipt *types.Receipt
		if err := c.Call(&txReceipt, "eth_getTransactionReceipt", txHash); err != nil {
			if fmt.Sprint(err) == "transaction indexing is in progress" {
				// `transaction indexing is in progress` is returned when
				// `eth_getTransactionCount` is called with no previously mined
				// blocks.
				time.Sleep(1 * time.Second)
			} else {
				return nil, err
			}
		}

		if txReceipt != nil {
			return txReceipt, nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	return nil, fmt.Errorf("timeout waiting tx hash=%v to be mined", txHash)
}

func main() {
	c, err := rpc.Dial("http://localhost:8545")

	if err != nil {
		log.Fatal("error dialing server", "url", "http://localhost:8545")
	}

	coinbase := getCoinbase(c)
	startingNonce := getTransactionCount(c, coinbase)

	var (
		wg  sync.WaitGroup
		sem = make(chan struct{}, 5)
	)

	for nonce := startingNonce; nonce <= startingNonce+15; nonce++ {
		nonce := nonce
		sem <- struct{}{}
		wg.Add(1)

		go func() {
			defer func() { <-sem }()
			defer wg.Done()

			if txHash, err := sendTransactionToSelf(c, coinbase, nonce); err != nil {
				log.Printf("error sending tx nonce=%v err=%v", uint64(nonce), err)
			} else if receipt, err := waitTransactionReceipt(c, txHash); err != nil {
				log.Printf("error waiting tx nonce=%v err=%v", uint64(nonce), err)
			} else {
				log.Printf("sent tx hash=%v nonce=%v blockNumber=%v", receipt.TxHash, uint64(nonce), receipt.BlockNumber)
			}
		}()
	}

	wg.Wait()
}
