# `go-ethereum@v1.13.4` dev-mode deadlock report

This repo contains logs/traces/etc. related to issue #TODO.

↓

#### System information

* Geth version
    ```
    Geth
    Version: 1.13.14-stable
    Git Commit: 2bd6bd01d2e8561dd7fc21b631f4a34ac16627a1
    Git Commit Date: 20240227
    Architecture: arm64
    Go Version: go1.21.0
    Operating System: darwin
    GOPATH=
    GOROOT=/Users/gashkov/.asdf/installs/golang/1.22.0/go
    ```
* CL client & version: none (dev-mode)
* OS & Version: macOS Sonoma 14.4.1 (23E224)
* Commit hash: not applicable

#### Steps to reproduce the behaviour

1. Run `go-ethereum` in a dev-mode with block generation on demand
2. Send a batch of transactions concurrently

<details>
<summary>Stress-testing script I used for reproducing this behavior.</summary>

```go
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
```
</details>
</details>
<details>
<summary>Stress-testing output</summary>

```
% go run main.go
2024/04/06 20:13:14 coinbase=0xfb8c69691E914275e19Eb9d42735844A120067aD
2024/04/06 20:13:14 nonce=0x0
2024/04/06 20:13:15 sent tx hash=0xad155b491b06bffa28d2e0ebd4cf4f85a8e5bec108428cb2492b0048395fc3b7 nonce=2 blockNumber=1
2024/04/06 20:13:15 sent tx hash=0x7c7d61049a6118838cb0babd50aa05d5f8feece647637313d2e432dc2448e6e4 nonce=1 blockNumber=1
2024/04/06 20:13:15 sent tx hash=0xa53f94890c86c1d20f6c6398c8cd07d8c91bad816c91b7119c525f597ae7237a nonce=0 blockNumber=1
2024/04/06 20:13:15 sent tx hash=0x06bbb3c89850442f73ac53fff35b42a529dac62dde0167480455dda21e4b54e4 nonce=4 blockNumber=1
2024/04/06 20:13:15 sent tx hash=0xb55986ad438a261b8225e6384eba83a3dd710f196a5105526f149d0c8f85c0a8 nonce=3 blockNumber=1
2024/04/06 20:13:15 sent tx hash=0xc2f5aa3063d679fd39d9f2495577cb204519e9b8ecd68065155083f9c13a9aae nonce=7 blockNumber=2
2024/04/06 20:13:15 sent tx hash=0xedb2c69e889019102540da5800c4717aa3e9178fa9f8a8e4ac260235827b122d nonce=5 blockNumber=2
2024/04/06 20:13:15 sent tx hash=0xcc36386a360e4c35ccf7da2cd6d87ec4509f619f6f1620a3a9e3c43d0a050f71 nonce=6 blockNumber=2
2024/04/06 20:13:15 sent tx hash=0xf7f8a37a85beb5d366c989295cd46961a9e2bb59fac54b58fcb69fd93c7caa25 nonce=8 blockNumber=2
2024/04/06 20:13:16 error waiting tx nonce=9 err=timeout waiting tx hash=0x9b54d66bb30c77334f95e2931ca4ab6f11f61b139af1a6fc517b3fb4fe8b5e93 to be mined
2024/04/06 20:13:16 error waiting tx nonce=11 err=timeout waiting tx hash=0x6bd2490d9e4539f37ef2493389cd1066a25b32b4b2f50076e2824437335ead8f to be mined
2024/04/06 20:13:16 error waiting tx nonce=13 err=timeout waiting tx hash=0x2298fea2beeae800aa07c9e1751de1a4995d3fc506e0af3fc9039d77e89309e9 to be mined
2024/04/06 20:13:16 error waiting tx nonce=10 err=timeout waiting tx hash=0x034b4941ac6c1de3fd8e32b5c9d9976fb5896c6dbd6f37e9bb019db9e136d825 to be mined
2024/04/06 20:13:16 error waiting tx nonce=12 err=timeout waiting tx hash=0x24ca4ec05d745c4271e7b4a70d7a74d403daa38306ef7a6f6ae01a8394db7964 to be mined
2024/04/06 20:13:17 error waiting tx nonce=14 err=timeout waiting tx hash=0x2120f1d967a28ba0c467fd6c638db89959d64a92872569e7e0869ba0f3abad43 to be mined
2024/04/06 20:13:17 error waiting tx nonce=15 err=timeout waiting tx hash=0xb0e0165e29eb44545cd852da79937ad50c5933bead69cf54fbc122b5b50163e6 to be mined
```

</details>

#### Expected behaviour

`geth --dev` accepts and executes transactions concurrently.

#### Actual behaviour

Transactions are accepted but stuck in the pool:

```
% curl \
  -s \
  -X POST http://localhost:8545/ \
  -H 'content-type: application/json' \
  -H 'accept: application/json, */*;q=0.5' \
  -d '{"jsonrpc":"2.0","id":"1","method":"txpool_inspect","params":[]}' | jq

{
  "jsonrpc": "2.0",
  "id": "1",
  "result": {
    "pending": {
      "0xfb8c69691E914275e19Eb9d42735844A120067aD": {
        "9": "0xfb8c69691E914275e19Eb9d42735844A120067aD: 0 wei + 21000 gas × 1750000001 wei"
      }
    },
    "queued": {
      "0xfb8c69691E914275e19Eb9d42735844A120067aD": {
        "10": "0xfb8c69691E914275e19Eb9d42735844A120067aD: 0 wei + 21000 gas × 1535240671 wei",
        "11": "0xfb8c69691E914275e19Eb9d42735844A120067aD: 0 wei + 21000 gas × 1535240671 wei",
        "12": "0xfb8c69691E914275e19Eb9d42735844A120067aD: 0 wei + 21000 gas × 1535240671 wei",
        "13": "0xfb8c69691E914275e19Eb9d42735844A120067aD: 0 wei + 21000 gas × 1535240671 wei",
        "14": "0xfb8c69691E914275e19Eb9d42735844A120067aD: 0 wei + 21000 gas × 1535240671 wei",
        "15": "0xfb8c69691E914275e19Eb9d42735844A120067aD: 0 wei + 21000 gas × 1535240671 wei"
      }
    }
  }
}
```

Looks like the root cause is the deadlock between `SimulatedBeacon` and
`TxPool` waiting each other. Overall, the following happens:

```
SimulatedBeacon.loop():
  case <-newTxs:
    SimulatedBeacon.SealBlock()
      TxPool.Sync()  // Runs pool reorg and waits until done
```

and at the same time

```
SubmitTransaction(newTx)
  EthAPIBackend.SendTx(newTx)
      LegacyPool.add(newTx)
        txFeed.send(newTx)
          newTxs <- newTx // Waits until SimulatedBeacon.loop will be ready
```

So, when new TXs are sent concurrently, `LegacyPool` waits `SimulatedBeacon`
to react to the new TX but the `SimulatedBeacon` itself waits `LegacyPool` to
sync itself.

#### Backtrace

Nothing too suspicious in logs, but stacktraces contain two goroutines
indirectly waiting each other though:

```
goroutine 4279 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/core/txpool.(*TxPool).Sync(0xc000513e60)
        /Users/gashkov/dev/go-ethereum/core/txpool/txpool.go:478 +0x148
github.com/ethereum/go-ethereum/eth/catalyst.(*ConsensusAPI).forkchoiceUpdated(0xc000178be0, {{0xc9, 0x10, 0x26, 0x77, 0xa1, 0x3, 0x24, 0x7, 0xab, ...}, ...}, ...)
        /Users/gashkov/dev/go-ethereum/eth/catalyst/api.go:397 +0x28d0
github.com/ethereum/go-ethereum/eth/catalyst.(*SimulatedBeacon).sealBlock(0xc0006626e0, {0x108c0da00, 0x0, 0x0}, 0x661190bc)
        /Users/gashkov/dev/go-ethereum/eth/catalyst/simulated_beacon.go:159 +0x4f4
github.com/ethereum/go-ethereum/eth/catalyst.(*SimulatedBeacon).Commit(0xc0006626e0)
        /Users/gashkov/dev/go-ethereum/eth/catalyst/simulated_beacon.go:249 +0xc4
github.com/ethereum/go-ethereum/eth/catalyst.(*api).loop(0xc0001b6980)
        /Users/gashkov/dev/go-ethereum/eth/catalyst/simulated_beacon_api.go:50 +0x2d8
created by github.com/ethereum/go-ethereum/eth/catalyst.RegisterSimulatedBeaconAPIs in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/catalyst/simulated_beacon.go:294 +0x14c
```
```
goroutine 4369 [select, 1 minutes]:
reflect.rselect({0xc00001d148, 0x2, 0x99?})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/runtime/select.go:589 +0x2d0
reflect.Select({0xc000000b40, 0x2, 0x5})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/reflect/value.go:3104 +0xa00
github.com/ethereum/go-ethereum/event.(*Feed).Send(0xc0003b9c90, {0x106d49200, 0xc000b78078})
        /Users/gashkov/dev/go-ethereum/event/feed.go:160 +0x758
github.com/ethereum/go-ethereum/core/txpool/legacypool.(*LegacyPool).runReorg(0xc0003b9c00, 0xc0005e8fc0, 0xc000488ed0, 0xc000458ce0, 0xc00089d290)
        /Users/gashkov/dev/go-ethereum/core/txpool/legacypool/legacypool.go:1336 +0xbf0
created by github.com/ethereum/go-ethereum/core/txpool/legacypool.(*LegacyPool).scheduleReorgLoop in goroutine 46
        /Users/gashkov/dev/go-ethereum/core/txpool/legacypool/legacypool.go:1205 +0x37c
```

<details>
<summary>All logs</summary>

```
GOROOT=/Users/gashkov/.asdf/installs/golang/1.21.0/go #gosetup
GOPATH=/Users/gashkov/.asdf/installs/golang/1.21.0/packages #gosetup
/Users/gashkov/.asdf/installs/golang/1.21.0/go/bin/go build -race -o /Users/gashkov/Library/Caches/JetBrains/GoLand2023.3/tmp/GoLand/___1go_build_github_com_ethereum_go_ethereum_cmd_geth -gcflags all=-N -l github.com/ethereum/go-ethereum/cmd/geth #gosetup
# github.com/karalabe/usb
In file included from /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/karalabe/usb@v0.0.2/libs.go:50:
/Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/karalabe/usb@v0.0.2/libusb/libusb/os/darwin_usb.c:53:29: warning: macro 'ATOMIC_VAR_INIT' has been marked as deprecated [-Wdeprecated-pragma]
/Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/lib/clang/15.0.0/include/stdatomic.h:54:41: note: macro marked 'deprecated' here
/Users/gashkov/Applications/GoLand.app/Contents/plugins/go-plugin/lib/dlv/macarm/dlv --listen=127.0.0.1:50626 --headless=true --api-version=2 --check-go-version=false --only-same-user=false exec /Users/gashkov/Library/Caches/JetBrains/GoLand2023.3/tmp/GoLand/___1go_build_github_com_ethereum_go_ethereum_cmd_geth -- --verbosity=5 --dev --http --http.addr=0.0.0.0 --http.vhosts=* --http.api=admin,debug,web3,eth,txpool,personal,miner,net,dev
API server listening at: 127.0.0.1:50626
debugserver-@(#)PROGRAM:LLDB  PROJECT:lldb-1500.0.404.7
 for arm64.
Got a connection, launched process /Users/gashkov/Library/Caches/JetBrains/GoLand2023.3/tmp/GoLand/___1go_build_github_com_ethereum_go_ethereum_cmd_geth (pid = 30418).
INFO [04-06|20:13:05.423] Starting Geth in ephemeral dev mode...
WARN [04-06|20:13:05.423] You are running Geth in --dev mode. Please note the following:

  1. This mode is only intended for fast, iterative development without assumptions on
     security or persistence.
  2. The database is created in memory unless specified otherwise. Therefore, shutting down
     your computer or losing power will wipe your entire block data and chain state for
     your dev environment.
  3. A random, pre-allocated developer account will be available and unlocked as
     eth.coinbase, which can be used for testing. The random dev account is temporary,
     stored on a ramdisk, and will be lost if your machine is restarted.
  4. Mining is enabled by default. However, the client will only seal blocks if transactions
     are pending in the mempool. The miner's minimum accepted gas price is 1.
  5. Networking is disabled; there is no listen-address, the maximum number of peers is set
     to 0, and discovery is disabled.

INFO [04-06|20:13:05.435] Maximum peer count                       ETH=50 total=50
DEBUG[04-06|20:13:05.463] FS scan times                            list="80.792µs" set="1.125µs" diff="1.791µs"
TRACE[04-06|20:13:05.463] Started watching keystore folder         path=/var/folders/qx/9y7_4y614wg1pnkhdcmpq_km0000gn/T/go-ethereum-keystore4071839221 folder=/var/folders/qx/9y7_4y614wg1pnkhdcmpq_km0000gn/T/go-ethereum-keystore4071839221
DEBUG[04-06|20:13:05.478] Sanitizing Go's GC trigger               percent=100
INFO [04-06|20:13:05.488] Set global gas cap                       cap=50,000,000
DEBUG[04-06|20:13:06.472] FS scan times                            list="81.458µs" set="25.833µs" diff="3.625µs"
TRACE[04-06|20:13:06.472] Handled keystore changes                 time="134.75µs"
INFO [04-06|20:13:06.927] Using developer account                  address=0xfb8c69691E914275e19Eb9d42735844A120067aD
INFO [04-06|20:13:06.929] Initializing the KZG library             backend=gokzg
INFO [04-06|20:13:07.129] Allocated trie memory caches             clean=154.00MiB dirty=256.00MiB
INFO [04-06|20:13:07.129] State schema set to default              scheme=hash
INFO [04-06|20:13:07.129] Initialising Ethereum protocol           network=1337 dbversion=<nil>
INFO [04-06|20:13:07.130] Writing custom genesis block
INFO [04-06|20:13:07.134] Persisted trie from memory database      nodes=13 size=1.91KiB time="152.709µs" gcnodes=0 gcsize=0.00B gctime=0s livenodes=0 livesize=0.00B
INFO [04-06|20:13:07.135]
INFO [04-06|20:13:07.135] ---------------------------------------------------------------------------------------------------------------------------------------------------------
INFO [04-06|20:13:07.135] Chain ID:  1337 (unknown)
INFO [04-06|20:13:07.135] Consensus: unknown
INFO [04-06|20:13:07.135]
INFO [04-06|20:13:07.135] Pre-Merge hard forks (block based):
INFO [04-06|20:13:07.135]  - Homestead:                   #0        (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/homestead.md)
INFO [04-06|20:13:07.135]  - Tangerine Whistle (EIP 150): #0        (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/tangerine-whistle.md)
INFO [04-06|20:13:07.135]  - Spurious Dragon/1 (EIP 155): #0        (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/spurious-dragon.md)
INFO [04-06|20:13:07.135]  - Spurious Dragon/2 (EIP 158): #0        (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/spurious-dragon.md)
INFO [04-06|20:13:07.135]  - Byzantium:                   #0        (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/byzantium.md)
INFO [04-06|20:13:07.135]  - Constantinople:              #0        (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/constantinople.md)
INFO [04-06|20:13:07.135]  - Petersburg:                  #0        (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/petersburg.md)
INFO [04-06|20:13:07.135]  - Istanbul:                    #0        (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/istanbul.md)
INFO [04-06|20:13:07.135]  - Muir Glacier:                #0        (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/muir-glacier.md)
INFO [04-06|20:13:07.135]  - Berlin:                      #0        (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/berlin.md)
INFO [04-06|20:13:07.135]  - London:                      #0        (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/london.md)
INFO [04-06|20:13:07.135]  - Arrow Glacier:               #0        (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/arrow-glacier.md)
INFO [04-06|20:13:07.135]  - Gray Glacier:                #0        (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/gray-glacier.md)
INFO [04-06|20:13:07.135]
INFO [04-06|20:13:07.135] Merge configured:
INFO [04-06|20:13:07.135]  - Hard-fork specification:    https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/paris.md
INFO [04-06|20:13:07.135]  - Network known to be merged: true
INFO [04-06|20:13:07.135]  - Total terminal difficulty:  0
INFO [04-06|20:13:07.135]
INFO [04-06|20:13:07.135] Post-Merge hard forks (timestamp based):
INFO [04-06|20:13:07.135]  - Shanghai:                    @0          (https://github.com/ethereum/execution-specs/blob/master/network-upgrades/mainnet-upgrades/shanghai.md)
INFO [04-06|20:13:07.135]
INFO [04-06|20:13:07.135] ---------------------------------------------------------------------------------------------------------------------------------------------------------
INFO [04-06|20:13:07.135]
INFO [04-06|20:13:07.136] Loaded most recent local block           number=0 hash=f0054e..040698 td=1 age=55y2w5d
WARN [04-06|20:13:07.136] Failed to load snapshot                  err="missing or corrupted snapshot"
INFO [04-06|20:13:07.136] Rebuilding state snapshot
DEBUG[04-06|20:13:07.136] Journalled generator progress            progress=empty
DEBUG[04-06|20:13:07.137] Start snapshot generation                root=292003..bb4642
INFO [04-06|20:13:07.137] Initialized transaction indexer          range="last 2350000 blocks"
INFO [04-06|20:13:07.137] Resuming state snapshot generation       root=292003..bb4642 accounts=0 slots=0 storage=0.00B dangling=0 elapsed="945.834µs"
TRACE[04-06|20:13:07.137] Detected outdated state range            kind=account prefix=0x61 last=0x err="wrong root: have 0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421 want 0x292003900d0d40ffa64b2748be0b3b8b5a8f53bb9ef6b0860fcc8ea2c9bb4642"
DEBUG[04-06|20:13:07.137] Regenerated state range                  kind=account prefix=0x61 root=292003..bb4642 last=0x count=10 created=10 updated=0 untouched=0 deleted=0
DEBUG[04-06|20:13:07.137] Journalled generator progress            progress=done
INFO [04-06|20:13:07.137] Generated state snapshot                 accounts=10 slots=0 storage=412.00B dangling=0 elapsed=1.745ms
DEBUG[04-06|20:13:07.141] Blobpool tip threshold updated           tip=1
INFO [04-06|20:13:07.143] Chain post-merge, sync via beacon client
INFO [04-06|20:13:07.143] Gasprice oracle is ignoring threshold set threshold=2
TRACE[04-06|20:13:07.143] Decrease miner recommit interval         from=2s to=2s
TRACE[04-06|20:13:07.145] Engine API request received              method=ForkchoiceUpdated head=f0054e..040698 finalized=f0054e..040698 safe=f0054e..040698
INFO [04-06|20:13:07.145] Entered PoS stage
INFO [04-06|20:13:07.145] Starting peer-to-peer node               instance=Geth/v1.13.14-stable-2bd6bd01/darwin-arm64/go1.21.0
WARN [04-06|20:13:07.145] P2P server will be useless, neither dialing nor listening
INFO [04-06|20:13:07.148] New local node record                    seq=1,712,427,187,146 id=6079679482be1efd ip=127.0.0.1 udp=0 tcp=0
INFO [04-06|20:13:07.148] Started P2P networking                   self=enode://a3e8bd63955696217e275827bd93e35d49ecd4515d7a47eadff62a570e4e115c62fcb35f231c0e2542cbaa3c84576bbb7912fd54b94128f9ba41389ca7af5b26@127.0.0.1:0
DEBUG[04-06|20:13:07.150] IPCs registered                          namespaces=admin,debug,web3,eth,txpool,miner,net,dev
INFO [04-06|20:13:07.150] IPC endpoint opened                      url=/var/folders/qx/9y7_4y614wg1pnkhdcmpq_km0000gn/T/geth.ipc
INFO [04-06|20:13:07.152] HTTP server started                      endpoint=[::]:8545 auth=false prefix= cors= vhosts=*
DEBUG[04-06|20:13:09.381] External IP changed                      ip="&{ch:0xc00018ea80 clock:{} timer:0xc0005e6230 deadline:148672697295416}" interface="UPNP IGDv1-IP1"
DEBUG[04-06|20:13:14.389] Served eth_coinbase                      conn=[::1]:50687 reqid=1 duration="280.333µs"
DEBUG[04-06|20:13:14.392] Served eth_getTransactionCount           conn=[::1]:50687 reqid=2 duration="468.833µs"
TRACE[04-06|20:13:14.396] Estimate gas usage automatically         gas=0x5208
TRACE[04-06|20:13:14.396] Estimate gas usage automatically         gas=0x5208
TRACE[04-06|20:13:14.396] Estimate gas usage automatically         gas=0x5208
TRACE[04-06|20:13:14.397] Estimate gas usage automatically         gas=0x5208
TRACE[04-06|20:13:14.398] Estimate gas usage automatically         gas=0x5208
INFO [04-06|20:13:14.401] Setting new local account                address=0xfb8c69691E914275e19Eb9d42735844A120067aD
TRACE[04-06|20:13:14.401] Pooled new future transaction            hash=ad155b..5fc3b7 from=0xfb8c69691E914275e19Eb9d42735844A120067aD to=0xfb8c69691E914275e19Eb9d42735844A120067aD
INFO [04-06|20:13:14.402] Submitted transaction                    hash=0xad155b491b06bffa28d2e0ebd4cf4f85a8e5bec108428cb2492b0048395fc3b7 from=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=2 recipient=0xfb8c69691E914275e19Eb9d42735844A120067aD value=0
DEBUG[04-06|20:13:14.402] Served eth_sendTransaction               conn=[::1]:50691 reqid=6 duration=6.606084ms
TRACE[04-06|20:13:14.403] Pooled new future transaction            hash=b55986..85c0a8 from=0xfb8c69691E914275e19Eb9d42735844A120067aD to=0xfb8c69691E914275e19Eb9d42735844A120067aD
INFO [04-06|20:13:14.403] Submitted transaction                    hash=0xb55986ad438a261b8225e6384eba83a3dd710f196a5105526f149d0c8f85c0a8 from=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=3 recipient=0xfb8c69691E914275e19Eb9d42735844A120067aD value=0
DEBUG[04-06|20:13:14.403] Served eth_sendTransaction               conn=[::1]:50690 reqid=7 duration=6.357542ms
TRACE[04-06|20:13:14.404] Pooled new future transaction            hash=7c7d61..48e6e4 from=0xfb8c69691E914275e19Eb9d42735844A120067aD to=0xfb8c69691E914275e19Eb9d42735844A120067aD
INFO [04-06|20:13:14.404] Submitted transaction                    hash=0x7c7d61049a6118838cb0babd50aa05d5f8feece647637313d2e432dc2448e6e4 from=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=1 recipient=0xfb8c69691E914275e19Eb9d42735844A120067aD value=0
DEBUG[04-06|20:13:14.405] Served eth_sendTransaction               conn=[::1]:50687 reqid=4 duration=10.634541ms
TRACE[04-06|20:13:14.405] Removed old queued transactions          count=0
TRACE[04-06|20:13:14.409] Removed unpayable queued transactions    count=0
TRACE[04-06|20:13:14.409] Promoted queued transactions             count=0
WARN [04-06|20:13:14.410] Served eth_getTransactionReceipt         conn=[::1]:50691 reqid=8 duration="264.084µs" err="transaction indexing is in progress" errdata="transaction indexing is in progress"
WARN [04-06|20:13:14.411] Served eth_getTransactionReceipt         conn=[::1]:50690 reqid=9 duration="164.709µs" err="transaction indexing is in progress" errdata="transaction indexing is in progress"
TRACE[04-06|20:13:14.411] Pooled new future transaction            hash=a53f94..e7237a from=0xfb8c69691E914275e19Eb9d42735844A120067aD to=0xfb8c69691E914275e19Eb9d42735844A120067aD
INFO [04-06|20:13:14.411] Submitted transaction                    hash=0xa53f94890c86c1d20f6c6398c8cd07d8c91bad816c91b7119c525f597ae7237a from=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=0 recipient=0xfb8c69691E914275e19Eb9d42735844A120067aD value=0
DEBUG[04-06|20:13:14.413] Served eth_sendTransaction               conn=[::1]:50689 reqid=5 duration=18.035708ms
TRACE[04-06|20:13:14.414] Pooled new future transaction            hash=06bbb3..4b54e4 from=0xfb8c69691E914275e19Eb9d42735844A120067aD to=0xfb8c69691E914275e19Eb9d42735844A120067aD
WARN [04-06|20:13:14.413] Served eth_getTransactionReceipt         conn=[::1]:50687 reqid=10 duration="183.834µs" err="transaction indexing is in progress" errdata="transaction indexing is in progress"
TRACE[04-06|20:13:14.414] Removed old queued transactions          count=0
TRACE[04-06|20:13:14.414] Removed unpayable queued transactions    count=0
TRACE[04-06|20:13:14.414] Promoted queued transactions             count=5
DEBUG[04-06|20:13:14.415] Distributed transactions                 plaintxs=5 blobtxs=0 largetxs=0 bcastpeers=0 bcastcount=0 annpeers=0 anncount=0
TRACE[04-06|20:13:14.415] Engine API request received              method=ForkchoiceUpdated head=f0054e..040698 finalized=f0054e..040698 safe=f0054e..040698
INFO [04-06|20:13:14.416] Submitted transaction                    hash=0x06bbb3c89850442f73ac53fff35b42a529dac62dde0167480455dda21e4b54e4 from=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=4 recipient=0xfb8c69691E914275e19Eb9d42735844A120067aD value=0
DEBUG[04-06|20:13:14.416] Served eth_sendTransaction               conn=[::1]:50688 reqid=3 duration=21.392959ms
DEBUG[04-06|20:13:14.418] Reinjecting stale transactions           count=0
WARN [04-06|20:13:14.419] Served eth_getTransactionReceipt         conn=[::1]:50690 reqid=11 duration="214.625µs" err="transaction indexing is in progress" errdata="transaction indexing is in progress"
WARN [04-06|20:13:14.420] Served eth_getTransactionReceipt         conn=[::1]:50688 reqid=12 duration="226.125µs" err="transaction indexing is in progress" errdata="transaction indexing is in progress"
INFO [04-06|20:13:14.422] Starting work on payload                 id=0x02b31fa2468f5390
TRACE[04-06|20:13:14.422] Engine API request received              method=GetPayload        id=0x02b31fa2468f5390
INFO [04-06|20:13:14.425] Updated payload                          id=0x02b31fa2468f5390 number=1 hash=4052dd..27bc10 txs=5 withdrawals=0 gas=105,000 fees=1.05e-13 root=89da34..5c888e elapsed=2.605ms
INFO [04-06|20:13:14.425] Stopping work on payload                 id=0x02b31fa2468f5390 reason=delivery
TRACE[04-06|20:13:14.425] Engine API request received              method=NewPayload        number=1 hash=4052dd..27bc10
TRACE[04-06|20:13:14.426] Inserting block without sethead          hash=4052dd..27bc10 number=0x105aaf260
INFO [04-06|20:13:14.430] Imported new potential chain segment     number=1           hash=4052dd..27bc10 blocks=1 txs=5 mgas=0.105 elapsed=4.039ms     mgasps=25.993 snapdiffs=109.00B triedirty=1.58KiB
TRACE[04-06|20:13:14.431] Engine API request received              method=ForkchoiceUpdated head=4052dd..27bc10 finalized=f0054e..040698 safe=4052dd..27bc10
INFO [04-06|20:13:14.432] Chain head was updated                   number=1           hash=4052dd..27bc10 root=89da34..5c888e elapsed="952.084µs"
DEBUG[04-06|20:13:14.433] Reinjecting stale transactions           count=0
TRACE[04-06|20:13:14.433] Removed old pending transaction          hash=a53f94..e7237a
TRACE[04-06|20:13:14.433] Removed old pending transaction          hash=7c7d61..48e6e4
TRACE[04-06|20:13:14.433] Removed old pending transaction          hash=ad155b..5fc3b7
TRACE[04-06|20:13:14.433] Removed old pending transaction          hash=b55986..85c0a8
TRACE[04-06|20:13:14.433] Removed old pending transaction          hash=06bbb3..4b54e4
TRACE[04-06|20:13:14.433] Decrease miner recommit interval         from=2s to=2s
INFO [04-06|20:13:14.433] Indexed transactions                     blocks=2 txs=5 tail=0 elapsed="845.625µs"
DEBUG[04-06|20:13:15.521] Served eth_getTransactionReceipt         conn=[::1]:50691 reqid=14 duration=5.789167ms
DEBUG[04-06|20:13:15.521] Served eth_getTransactionReceipt         conn=[::1]:50690 reqid=13 duration=6.218459ms
DEBUG[04-06|20:13:15.523] Served eth_getTransactionReceipt         conn=[::1]:50695 reqid=15 duration=1.635833ms
DEBUG[04-06|20:13:15.526] Served eth_getTransactionReceipt         conn=[::1]:50696 reqid=16 duration=2.694041ms
DEBUG[04-06|20:13:15.529] Served eth_getTransactionReceipt         conn=[::1]:50697 reqid=18 duration=2.560041ms
TRACE[04-06|20:13:15.530] Estimate gas usage automatically         gas=0x5208
TRACE[04-06|20:13:15.533] Estimate gas usage automatically         gas=0x5208
TRACE[04-06|20:13:15.533] Estimate gas usage automatically         gas=0x5208
TRACE[04-06|20:13:15.533] Pooled new future transaction            hash=edb2c6..7b122d from=0xfb8c69691E914275e19Eb9d42735844A120067aD to=0xfb8c69691E914275e19Eb9d42735844A120067aD
INFO [04-06|20:13:15.534] Submitted transaction                    hash=0xedb2c69e889019102540da5800c4717aa3e9178fa9f8a8e4ac260235827b122d from=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=5 recipient=0xfb8c69691E914275e19Eb9d42735844A120067aD value=0
TRACE[04-06|20:13:15.534] Removed old queued transactions          count=0
DEBUG[04-06|20:13:15.534] Served eth_sendTransaction               conn=[::1]:50691 reqid=17 duration=6.051583ms
TRACE[04-06|20:13:15.534] Removed unpayable queued transactions    count=0
TRACE[04-06|20:13:15.535] Promoted queued transactions             count=1
DEBUG[04-06|20:13:15.535] Distributed transactions                 plaintxs=1 blobtxs=0 largetxs=0 bcastpeers=0 bcastcount=0 annpeers=0 anncount=0
TRACE[04-06|20:13:15.535] Engine API request received              method=ForkchoiceUpdated head=4052dd..27bc10 finalized=f0054e..040698 safe=4052dd..27bc10
TRACE[04-06|20:13:15.536] Pooled new future transaction            hash=cc3638..050f71 from=0xfb8c69691E914275e19Eb9d42735844A120067aD to=0xfb8c69691E914275e19Eb9d42735844A120067aD
INFO [04-06|20:13:15.536] Submitted transaction                    hash=0xcc36386a360e4c35ccf7da2cd6d87ec4509f619f6f1620a3a9e3c43d0a050f71 from=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=6 recipient=0xfb8c69691E914275e19Eb9d42735844A120067aD value=0
DEBUG[04-06|20:13:15.536] Served eth_sendTransaction               conn=[::1]:50695 reqid=19 duration=4.931ms
TRACE[04-06|20:13:15.536] Estimate gas usage automatically         gas=0x5208
TRACE[04-06|20:13:15.536] Pooled new future transaction            hash=c2f5aa..3a9aae from=0xfb8c69691E914275e19Eb9d42735844A120067aD to=0xfb8c69691E914275e19Eb9d42735844A120067aD
INFO [04-06|20:13:15.537] Submitted transaction                    hash=0xc2f5aa3063d679fd39d9f2495577cb204519e9b8ecd68065155083f9c13a9aae from=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=7 recipient=0xfb8c69691E914275e19Eb9d42735844A120067aD value=0
DEBUG[04-06|20:13:15.537] Served eth_sendTransaction               conn=[::1]:50696 reqid=20 duration=4.942708ms
DEBUG[04-06|20:13:15.537] Reinjecting stale transactions           count=0
TRACE[04-06|20:13:15.537] Removed old queued transactions          count=0
TRACE[04-06|20:13:15.537] Removed unpayable queued transactions    count=0
TRACE[04-06|20:13:15.537] Promoted queued transactions             count=0
TRACE[04-06|20:13:15.538] Pooled new future transaction            hash=f7f8a3..7caa25 from=0xfb8c69691E914275e19Eb9d42735844A120067aD to=0xfb8c69691E914275e19Eb9d42735844A120067aD
TRACE[04-06|20:13:15.539] Removed old queued transactions          count=0
TRACE[04-06|20:13:15.539] Removed unpayable queued transactions    count=0
TRACE[04-06|20:13:15.539] Promoted queued transactions             count=3
INFO [04-06|20:13:15.539] Submitted transaction                    hash=0xf7f8a37a85beb5d366c989295cd46961a9e2bb59fac54b58fcb69fd93c7caa25 from=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=8 recipient=0xfb8c69691E914275e19Eb9d42735844A120067aD value=0
DEBUG[04-06|20:13:15.539] Served eth_sendTransaction               conn=[::1]:50697 reqid=21 duration=3.876541ms
DEBUG[04-06|20:13:15.540] Distributed transactions                 plaintxs=3 blobtxs=0 largetxs=0 bcastpeers=0 bcastcount=0 annpeers=0 anncount=0
DEBUG[04-06|20:13:15.542] Served eth_getTransactionReceipt         conn=[::1]:50696 reqid=25 duration="135.625µs"
DEBUG[04-06|20:13:15.542] Served eth_getTransactionReceipt         conn=[::1]:50691 reqid=23 duration="154.917µs"
INFO [04-06|20:13:15.541] Starting work on payload                 id=0x02b343cad404c4b5
DEBUG[04-06|20:13:15.544] Served eth_getTransactionReceipt         conn=[::1]:50695 reqid=24 duration=2.126041ms
TRACE[04-06|20:13:15.544] Engine API request received              method=GetPayload        id=0x02b343cad404c4b5
TRACE[04-06|20:13:15.544] Estimate gas usage automatically         gas=0x5208
TRACE[04-06|20:13:15.545] Pooled new future transaction            hash=9b54d6..8b5e93 from=0xfb8c69691E914275e19Eb9d42735844A120067aD to=0xfb8c69691E914275e19Eb9d42735844A120067aD
INFO [04-06|20:13:15.546] Submitted transaction                    hash=0x9b54d66bb30c77334f95e2931ca4ab6f11f61b139af1a6fc517b3fb4fe8b5e93 from=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=9 recipient=0xfb8c69691E914275e19Eb9d42735844A120067aD value=0
DEBUG[04-06|20:13:15.546] Served eth_sendTransaction               conn=[::1]:50690 reqid=22 duration=3.769958ms
DEBUG[04-06|20:13:15.547] Served eth_getTransactionReceipt         conn=[::1]:50695 reqid=26 duration="131.375µs"
INFO [04-06|20:13:15.547] Updated payload                          id=0x02b343cad404c4b5 number=2           hash=c91026..4af5b9 txs=4 withdrawals=0 gas=84000   fees=8.4e-14  root=9b54af..08e71e elapsed=2.782ms
INFO [04-06|20:13:15.547] Stopping work on payload                 id=0x02b343cad404c4b5 reason=delivery
TRACE[04-06|20:13:15.547] Engine API request received              method=NewPayload        number=2           hash=c91026..4af5b9
TRACE[04-06|20:13:15.548] Inserting block without sethead          hash=c91026..4af5b9 number=0x105aaf260
DEBUG[04-06|20:13:15.549] Served eth_getTransactionReceipt         conn=[::1]:50695 reqid=27 duration="104.625µs"
INFO [04-06|20:13:15.552] Imported new potential chain segment     number=2           hash=c91026..4af5b9 blocks=1 txs=4 mgas=0.084 elapsed=3.917ms     mgasps=21.443 snapdiffs=218.00B triedirty=2.92KiB
TRACE[04-06|20:13:15.552] Engine API request received              method=ForkchoiceUpdated head=c91026..4af5b9 finalized=f0054e..040698 safe=c91026..4af5b9
INFO [04-06|20:13:15.554] Chain head was updated                   number=2           hash=c91026..4af5b9 root=9b54af..08e71e elapsed="701.042µs"
TRACE[04-06|20:13:15.554] Engine API request received              method=ForkchoiceUpdated head=c91026..4af5b9 finalized=f0054e..040698 safe=c91026..4af5b9
DEBUG[04-06|20:13:15.554] Reinjecting stale transactions           count=0
TRACE[04-06|20:13:15.554] Skipping transaction with low nonce      hash=edb2c6..7b122d sender=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=5
TRACE[04-06|20:13:15.554] Removed old queued transactions          count=0
TRACE[04-06|20:13:15.554] Removed unpayable queued transactions    count=0
TRACE[04-06|20:13:15.555] Promoted queued transactions             count=1
TRACE[04-06|20:13:15.555] Skipping transaction with low nonce      hash=cc3638..050f71 sender=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=6
TRACE[04-06|20:13:15.555] Removed old pending transaction          hash=edb2c6..7b122d
TRACE[04-06|20:13:15.555] Removed old pending transaction          hash=cc3638..050f71
TRACE[04-06|20:13:15.555] Removed old pending transaction          hash=c2f5aa..3a9aae
TRACE[04-06|20:13:15.555] Removed old pending transaction          hash=f7f8a3..7caa25
TRACE[04-06|20:13:15.555] Skipping transaction with low nonce      hash=c2f5aa..3a9aae sender=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=7
DEBUG[04-06|20:13:15.555] Distributed transactions                 plaintxs=1 blobtxs=0 largetxs=0 bcastpeers=0 bcastcount=0 annpeers=0 anncount=0
TRACE[04-06|20:13:15.555] Skipping transaction with low nonce      hash=f7f8a3..7caa25 sender=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=8
TRACE[04-06|20:13:15.555] Decrease miner recommit interval         from=2s to=2s
DEBUG[04-06|20:13:15.647] Served eth_getTransactionReceipt         conn=[::1]:50695 reqid=28 duration=1.086333ms
DEBUG[04-06|20:13:15.647] Served eth_getTransactionReceipt         conn=[::1]:50691 reqid=29 duration="989.5µs"
DEBUG[04-06|20:13:15.651] Served eth_getTransactionReceipt         conn=[::1]:50698 reqid=30 duration=1.518208ms
DEBUG[04-06|20:13:15.651] Served eth_getTransactionReceipt         conn=[::1]:50699 reqid=31 duration=1.343917ms
TRACE[04-06|20:13:15.651] Estimate gas usage automatically         gas=0x5208
TRACE[04-06|20:13:15.652] Estimate gas usage automatically         gas=0x5208
TRACE[04-06|20:13:15.652] Pooled new future transaction            hash=034b49..36d825 from=0xfb8c69691E914275e19Eb9d42735844A120067aD to=0xfb8c69691E914275e19Eb9d42735844A120067aD
INFO [04-06|20:13:15.652] Submitted transaction                    hash=0x034b4941ac6c1de3fd8e32b5c9d9976fb5896c6dbd6f37e9bb019db9e136d825 from=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=10 recipient=0xfb8c69691E914275e19Eb9d42735844A120067aD value=0
DEBUG[04-06|20:13:15.652] Served eth_sendTransaction               conn=[::1]:50691 reqid=32 duration=1.678ms
TRACE[04-06|20:13:15.653] Pooled new future transaction            hash=6bd249..5ead8f from=0xfb8c69691E914275e19Eb9d42735844A120067aD to=0xfb8c69691E914275e19Eb9d42735844A120067aD
INFO [04-06|20:13:15.653] Submitted transaction                    hash=0x6bd2490d9e4539f37ef2493389cd1066a25b32b4b2f50076e2824437335ead8f from=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=11 recipient=0xfb8c69691E914275e19Eb9d42735844A120067aD value=0
DEBUG[04-06|20:13:15.653] Served eth_sendTransaction               conn=[::1]:50695 reqid=33 duration=1.654708ms
TRACE[04-06|20:13:15.653] Estimate gas usage automatically         gas=0x5208
TRACE[04-06|20:13:15.653] Estimate gas usage automatically         gas=0x5208
DEBUG[04-06|20:13:15.654] Served eth_getTransactionReceipt         conn=[::1]:50691 reqid=36 duration="66.083µs"
TRACE[04-06|20:13:15.654] Pooled new future transaction            hash=24ca4e..db7964 from=0xfb8c69691E914275e19Eb9d42735844A120067aD to=0xfb8c69691E914275e19Eb9d42735844A120067aD
INFO [04-06|20:13:15.654] Submitted transaction                    hash=0x24ca4ec05d745c4271e7b4a70d7a74d403daa38306ef7a6f6ae01a8394db7964 from=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=12 recipient=0xfb8c69691E914275e19Eb9d42735844A120067aD value=0
DEBUG[04-06|20:13:15.654] Served eth_sendTransaction               conn=[::1]:50699 reqid=34 duration=1.53575ms
TRACE[04-06|20:13:15.654] Pooled new future transaction            hash=2298fe..9309e9 from=0xfb8c69691E914275e19Eb9d42735844A120067aD to=0xfb8c69691E914275e19Eb9d42735844A120067aD
INFO [04-06|20:13:15.654] Submitted transaction                    hash=0x2298fea2beeae800aa07c9e1751de1a4995d3fc506e0af3fc9039d77e89309e9 from=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=13 recipient=0xfb8c69691E914275e19Eb9d42735844A120067aD value=0
DEBUG[04-06|20:13:15.654] Served eth_getTransactionReceipt         conn=[::1]:50700 reqid=37 duration="56.042µs"
DEBUG[04-06|20:13:15.654] Served eth_sendTransaction               conn=[::1]:50698 reqid=35 duration=1.871958ms
DEBUG[04-06|20:13:15.655] Served eth_getTransactionReceipt         conn=[::1]:50691 reqid=38 duration="61.583µs"
DEBUG[04-06|20:13:15.655] Served eth_getTransactionReceipt         conn=[::1]:50701 reqid=39 duration="48.417µs"
DEBUG[04-06|20:13:15.656] Served eth_getTransactionReceipt         conn=[::1]:50700 reqid=40 duration="52.875µs"
DEBUG[04-06|20:13:15.755] Served eth_getTransactionReceipt         conn=[::1]:50691 reqid=41 duration="69.583µs"
DEBUG[04-06|20:13:15.756] Served eth_getTransactionReceipt         conn=[::1]:50701 reqid=42 duration="57.25µs"
DEBUG[04-06|20:13:15.756] Served eth_getTransactionReceipt         conn=[::1]:50691 reqid=43 duration="44.041µs"
DEBUG[04-06|20:13:15.757] Served eth_getTransactionReceipt         conn=[::1]:50702 reqid=44 duration="60.125µs"
DEBUG[04-06|20:13:15.758] Served eth_getTransactionReceipt         conn=[::1]:50691 reqid=45 duration="47.75µs"
DEBUG[04-06|20:13:15.858] Served eth_getTransactionReceipt         conn=[::1]:50702 reqid=46 duration="71.166µs"
DEBUG[04-06|20:13:15.858] Served eth_getTransactionReceipt         conn=[::1]:50701 reqid=47 duration="55.875µs"
DEBUG[04-06|20:13:15.859] Served eth_getTransactionReceipt         conn=[::1]:50703 reqid=48 duration="54.334µs"
DEBUG[04-06|20:13:15.859] Served eth_getTransactionReceipt         conn=[::1]:50702 reqid=49 duration="56.709µs"
DEBUG[04-06|20:13:15.860] Served eth_getTransactionReceipt         conn=[::1]:50701 reqid=50 duration="52.5µs"
DEBUG[04-06|20:13:15.960] Served eth_getTransactionReceipt         conn=[::1]:50702 reqid=51 duration="108.708µs"
DEBUG[04-06|20:13:15.961] Served eth_getTransactionReceipt         conn=[::1]:50703 reqid=52 duration="58.208µs"
DEBUG[04-06|20:13:15.962] Served eth_getTransactionReceipt         conn=[::1]:50702 reqid=53 duration="54.209µs"
DEBUG[04-06|20:13:15.963] Served eth_getTransactionReceipt         conn=[::1]:50712 reqid=54 duration="49.708µs"
DEBUG[04-06|20:13:15.963] Served eth_getTransactionReceipt         conn=[::1]:50713 reqid=55 duration="50.125µs"
DEBUG[04-06|20:13:16.063] Served eth_getTransactionReceipt         conn=[::1]:50703 reqid=57 duration="82.083µs"
DEBUG[04-06|20:13:16.063] Served eth_getTransactionReceipt         conn=[::1]:50712 reqid=56 duration="79.791µs"
DEBUG[04-06|20:13:16.065] Served eth_getTransactionReceipt         conn=[::1]:50714 reqid=58 duration="55.916µs"
DEBUG[04-06|20:13:16.065] Served eth_getTransactionReceipt         conn=[::1]:50715 reqid=59 duration="62.917µs"
DEBUG[04-06|20:13:16.066] Served eth_getTransactionReceipt         conn=[::1]:50703 reqid=60 duration="149.458µs"
DEBUG[04-06|20:13:16.167] Served eth_getTransactionReceipt         conn=[::1]:50716 reqid=63 duration="80.458µs"
DEBUG[04-06|20:13:16.167] Served eth_getTransactionReceipt         conn=[::1]:50712 reqid=61 duration="92.458µs"
DEBUG[04-06|20:13:16.168] Served eth_getTransactionReceipt         conn=[::1]:50717 reqid=62 duration="58.709µs"
DEBUG[04-06|20:13:16.168] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=64 duration="55.625µs"
DEBUG[04-06|20:13:16.169] Served eth_getTransactionReceipt         conn=[::1]:50716 reqid=65 duration="50.875µs"
DEBUG[04-06|20:13:16.269] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=66 duration="86.417µs"
DEBUG[04-06|20:13:16.269] Served eth_getTransactionReceipt         conn=[::1]:50712 reqid=67 duration="86.167µs"
DEBUG[04-06|20:13:16.271] Served eth_getTransactionReceipt         conn=[::1]:50720 reqid=69 duration="57.375µs"
DEBUG[04-06|20:13:16.271] Served eth_getTransactionReceipt         conn=[::1]:50719 reqid=68 duration="56.917µs"
DEBUG[04-06|20:13:16.272] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=70 duration="61.25µs"
DEBUG[04-06|20:13:16.373] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=72 duration="70µs"
DEBUG[04-06|20:13:16.373] Served eth_getTransactionReceipt         conn=[::1]:50712 reqid=71 duration="69.625µs"
DEBUG[04-06|20:13:16.375] Served eth_getTransactionReceipt         conn=[::1]:50721 reqid=73 duration="58.584µs"
DEBUG[04-06|20:13:16.375] Served eth_getTransactionReceipt         conn=[::1]:50723 reqid=74 duration="54.375µs"
DEBUG[04-06|20:13:16.375] Served eth_getTransactionReceipt         conn=[::1]:50722 reqid=75 duration="58.333µs"
DEBUG[04-06|20:13:16.477] Served eth_getTransactionReceipt         conn=[::1]:50712 reqid=76 duration="93.25µs"
DEBUG[04-06|20:13:16.477] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=77 duration="103µs"
DEBUG[04-06|20:13:16.478] Served eth_getTransactionReceipt         conn=[::1]:50725 reqid=78 duration="60.375µs"
DEBUG[04-06|20:13:16.479] Served eth_getTransactionReceipt         conn=[::1]:50724 reqid=80 duration="50.416µs"
DEBUG[04-06|20:13:16.479] Served eth_getTransactionReceipt         conn=[::1]:50726 reqid=79 duration="311.083µs"
DEBUG[04-06|20:13:16.581] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=82 duration="92.625µs"
DEBUG[04-06|20:13:16.581] Served eth_getTransactionReceipt         conn=[::1]:50712 reqid=81 duration="83.208µs"
DEBUG[04-06|20:13:16.582] Served eth_getTransactionReceipt         conn=[::1]:50728 reqid=84 duration="79.791µs"
DEBUG[04-06|20:13:16.582] Served eth_getTransactionReceipt         conn=[::1]:50729 reqid=85 duration="65.041µs"
TRACE[04-06|20:13:16.583] Estimate gas usage automatically         gas=0x5208
TRACE[04-06|20:13:16.584] Pooled new future transaction            hash=2120f1..abad43 from=0xfb8c69691E914275e19Eb9d42735844A120067aD to=0xfb8c69691E914275e19Eb9d42735844A120067aD
INFO [04-06|20:13:16.584] Submitted transaction                    hash=0x2120f1d967a28ba0c467fd6c638db89959d64a92872569e7e0869ba0f3abad43 from=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=14 recipient=0xfb8c69691E914275e19Eb9d42735844A120067aD value=0
DEBUG[04-06|20:13:16.584] Served eth_sendTransaction               conn=[::1]:50727 reqid=83 duration=1.814792ms
DEBUG[04-06|20:13:16.586] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=86 duration="62.75µs"
TRACE[04-06|20:13:16.685] Estimate gas usage automatically         gas=0x5208
TRACE[04-06|20:13:16.686] Pooled new future transaction            hash=b0e016..0163e6 from=0xfb8c69691E914275e19Eb9d42735844A120067aD to=0xfb8c69691E914275e19Eb9d42735844A120067aD
INFO [04-06|20:13:16.686] Submitted transaction                    hash=0xb0e0165e29eb44545cd852da79937ad50c5933bead69cf54fbc122b5b50163e6 from=0xfb8c69691E914275e19Eb9d42735844A120067aD nonce=15 recipient=0xfb8c69691E914275e19Eb9d42735844A120067aD value=0
DEBUG[04-06|20:13:16.686] Served eth_sendTransaction               conn=[::1]:50718 reqid=87 duration=1.819958ms
DEBUG[04-06|20:13:16.688] Served eth_getTransactionReceipt         conn=[::1]:50712 reqid=88 duration="71.125µs"
DEBUG[04-06|20:13:16.688] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=89 duration="71.167µs"
DEBUG[04-06|20:13:16.790] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=90 duration="76.166µs"
DEBUG[04-06|20:13:16.791] Served eth_getTransactionReceipt         conn=[::1]:50712 reqid=91 duration="60.833µs"
DEBUG[04-06|20:13:16.892] Served eth_getTransactionReceipt         conn=[::1]:50712 reqid=93 duration="67.917µs"
DEBUG[04-06|20:13:16.892] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=92 duration="113.75µs"
DEBUG[04-06|20:13:16.995] Served eth_getTransactionReceipt         conn=[::1]:50712 reqid=95 duration="63.209µs"
DEBUG[04-06|20:13:16.995] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=94 duration="63.75µs"
DEBUG[04-06|20:13:17.097] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=97 duration="91.875µs"
DEBUG[04-06|20:13:17.097] Served eth_getTransactionReceipt         conn=[::1]:50712 reqid=96 duration="98.625µs"
DEBUG[04-06|20:13:17.200] Served eth_getTransactionReceipt         conn=[::1]:50712 reqid=98 duration="86.375µs"
DEBUG[04-06|20:13:17.200] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=99 duration="83.458µs"
DEBUG[04-06|20:13:17.304] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=101 duration="84.334µs"
DEBUG[04-06|20:13:17.304] Served eth_getTransactionReceipt         conn=[::1]:50712 reqid=100 duration="92.375µs"
DEBUG[04-06|20:13:17.407] Served eth_getTransactionReceipt         conn=[::1]:50712 reqid=103 duration="81µs"
DEBUG[04-06|20:13:17.407] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=102 duration="83µs"
DEBUG[04-06|20:13:17.509] Served eth_getTransactionReceipt         conn=[::1]:50712 reqid=104 duration="76.042µs"
DEBUG[04-06|20:13:17.509] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=105 duration="71.625µs"
DEBUG[04-06|20:13:17.612] Served eth_getTransactionReceipt         conn=[::1]:50718 reqid=106 duration="95.416µs"
DEBUG[04-06|20:13:23.138] Transaction pool status report           executable=1 queued=6 stales=0
```
</details>
<details>
<summary>All stacktraces</summary>

```
goroutine 4556 [running]:
runtime/pprof.writeGoroutineStacks({0x106f578a0, 0xc0005ea270})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/runtime/pprof/pprof.go:703 +0xb0
runtime/pprof.writeGoroutine({0x106f578a0, 0xc0005ea270}, 0x2)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/runtime/pprof/pprof.go:692 +0x5c
runtime/pprof.(*Profile).WriteTo(0x107ab4320, {0x106f578a0, 0xc0005ea270}, 0x2)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/runtime/pprof/pprof.go:329 +0xd4
github.com/ethereum/go-ethereum/internal/debug.(*HandlerT).Stacks(0xc000723b58, 0x0)
        /Users/gashkov/dev/go-ethereum/internal/debug/api.go:194 +0x94
reflect.Value.call({0xc00029cfa0, 0xc000a89068, 0x13}, {0x10640291a, 0x4}, {0xc000c84550, 0x2, 0x3})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/reflect/value.go:596 +0xcb0
reflect.Value.Call({0xc00029cfa0, 0xc000a89068, 0x13}, {0xc000c84550, 0x2, 0x3})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/reflect/value.go:380 +0xc4
github.com/ethereum/go-ethereum/rpc.(*callback).call(0xc000b88840, {0x106f66790, 0xc000c84500}, {0xc000e140b4, 0xc}, {0xc0009ae168, 0x1, 0x1})
        /Users/gashkov/dev/go-ethereum/rpc/service.go:205 +0x6f8
github.com/ethereum/go-ethereum/rpc.(*handler).runMethod(0xc000ac6140, {0x106f66790, 0xc000c84500}, 0xc0004343f0, 0xc000b88840, {0xc0009ae168, 0x1, 0x1})
        /Users/gashkov/dev/go-ethereum/rpc/handler.go:565 +0xc8
github.com/ethereum/go-ethereum/rpc.(*handler).handleCall(0xc000ac6140, 0xc0005ea180, 0xc0004343f0)
        /Users/gashkov/dev/go-ethereum/rpc/handler.go:512 +0x384
github.com/ethereum/go-ethereum/rpc.(*handler).handleCallMsg(0xc000ac6140, 0xc0005ea180, 0xc0004343f0)
        /Users/gashkov/dev/go-ethereum/rpc/handler.go:470 +0x2b8
github.com/ethereum/go-ethereum/rpc.(*handler).handleNonBatchCall(0xc000ac6140, 0xc0005ea180, 0xc0004343f0)
        /Users/gashkov/dev/go-ethereum/rpc/handler.go:296 +0x3c8
github.com/ethereum/go-ethereum/rpc.(*handler).handleMsg.func1.1(0xc0005ea180)
        /Users/gashkov/dev/go-ethereum/rpc/handler.go:269 +0x5c
github.com/ethereum/go-ethereum/rpc.(*handler).startCallProc.func1()
        /Users/gashkov/dev/go-ethereum/rpc/handler.go:387 +0x238
created by github.com/ethereum/go-ethereum/rpc.(*handler).startCallProc in goroutine 4625
        /Users/gashkov/dev/go-ethereum/rpc/handler.go:383 +0x128

goroutine 1 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/node.(*Node).Wait(0xc0004600e0)
        /Users/gashkov/dev/go-ethereum/node/node.go:557 +0x58
main.geth(0xc000690200)
        /Users/gashkov/dev/go-ethereum/cmd/geth/main.go:344 +0x2e8
github.com/urfave/cli/v2.(*Command).Run(0xc0002bfb80, 0xc000690200, {0xc0001d8000, 0x7, 0x7})
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/urfave/cli/v2@v2.25.7/command.go:274 +0x1314
github.com/urfave/cli/v2.(*App).RunContext(0xc00013a5a0, {0x106f66838, 0x108c0da00}, {0xc0001d8000, 0x7, 0x7})
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/urfave/cli/v2@v2.25.7/app.go:332 +0x300
github.com/urfave/cli/v2.(*App).Run(0xc00013a5a0, {0xc0001d8000, 0x7, 0x7})
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/urfave/cli/v2@v2.25.7/app.go:309 +0x88
main.main()
        /Users/gashkov/dev/go-ethereum/cmd/geth/main.go:270 +0x7c

goroutine 34 [syscall, 1 minutes]:
os/signal.signal_recv()
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/runtime/sigqueue.go:149 +0x2c
os/signal.loop()
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/os/signal/signal_unix.go:23 +0x30
created by os/signal.Notify.func1.1 in goroutine 1
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/os/signal/signal.go:151 +0x58

goroutine 23 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/core.(*txSenderCacher).cache(0xc0003947f0)
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:63 +0x6c
created by github.com/ethereum/go-ethereum/core.newTxSenderCacher in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:55 +0x188

goroutine 24 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/core.(*txSenderCacher).cache(0xc0003947f0)
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:63 +0x6c
created by github.com/ethereum/go-ethereum/core.newTxSenderCacher in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:55 +0x188

goroutine 25 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/core.(*txSenderCacher).cache(0xc0003947f0)
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:63 +0x6c
created by github.com/ethereum/go-ethereum/core.newTxSenderCacher in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:55 +0x188

goroutine 26 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/core.(*txSenderCacher).cache(0xc0003947f0)
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:63 +0x6c
created by github.com/ethereum/go-ethereum/core.newTxSenderCacher in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:55 +0x188

goroutine 27 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/core.(*txSenderCacher).cache(0xc0003947f0)
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:63 +0x6c
created by github.com/ethereum/go-ethereum/core.newTxSenderCacher in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:55 +0x188

goroutine 28 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/core.(*txSenderCacher).cache(0xc0003947f0)
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:63 +0x6c
created by github.com/ethereum/go-ethereum/core.newTxSenderCacher in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:55 +0x188

goroutine 29 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/core.(*txSenderCacher).cache(0xc0003947f0)
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:63 +0x6c
created by github.com/ethereum/go-ethereum/core.newTxSenderCacher in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:55 +0x188

goroutine 30 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/core.(*txSenderCacher).cache(0xc0003947f0)
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:63 +0x6c
created by github.com/ethereum/go-ethereum/core.newTxSenderCacher in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:55 +0x188

goroutine 31 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/core.(*txSenderCacher).cache(0xc0003947f0)
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:63 +0x6c
created by github.com/ethereum/go-ethereum/core.newTxSenderCacher in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:55 +0x188

goroutine 32 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/core.(*txSenderCacher).cache(0xc0003947f0)
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:63 +0x6c
created by github.com/ethereum/go-ethereum/core.newTxSenderCacher in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:55 +0x188

goroutine 33 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/core.(*txSenderCacher).cache(0xc0003947f0)
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:63 +0x6c
created by github.com/ethereum/go-ethereum/core.newTxSenderCacher in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:55 +0x188

goroutine 66 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/core.(*txSenderCacher).cache(0xc0003947f0)
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:63 +0x6c
created by github.com/ethereum/go-ethereum/core.newTxSenderCacher in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/sender_cacher.go:55 +0x188

goroutine 40 [syscall, 1 minutes]:
syscall.syscall6(0x1063c4ed4?, 0x109205100?, 0xc000021528?, 0x104403714?, 0x0?, 0xc000048190?, 0xc000021538?)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/runtime/sys_darwin.go:45 +0x68
golang.org/x/sys/unix.kevent(0x9, 0x0, 0x0, 0xc000021e58, 0xa, 0x0)
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/golang.org/x/sys@v0.16.0/unix/zsyscall_darwin_arm64.go:275 +0xc4
golang.org/x/sys/unix.Kevent(0x9, {0x0, 0x0, 0x0}, {0xc000021e58, 0xa, 0xa}, 0x0)
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/golang.org/x/sys@v0.16.0/unix/syscall_bsd.go:397 +0x114
github.com/fsnotify/fsnotify.(*Watcher).read(0xc000434070, {0xc000021e58, 0xa, 0xa})
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/fsnotify/fsnotify@v1.6.0/backend_kqueue.go:702 +0xb4
github.com/fsnotify/fsnotify.(*Watcher).readEvents(0xc000434070)
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/fsnotify/fsnotify@v1.6.0/backend_kqueue.go:487 +0x174
created by github.com/fsnotify/fsnotify.NewWatcher in goroutine 9
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/fsnotify/fsnotify@v1.6.0/backend_kqueue.go:155 +0x5a4

goroutine 8 [select, 1 minutes]:
github.com/ethereum/go-ethereum/accounts.(*Manager).update(0xc000460460)
        /Users/gashkov/dev/go-ethereum/accounts/manager.go:137 +0x218
created by github.com/ethereum/go-ethereum/accounts.NewManager in goroutine 1
        /Users/gashkov/dev/go-ethereum/accounts/manager.go:94 +0xa8c

goroutine 9 [select, 1 minutes]:
github.com/ethereum/go-ethereum/accounts/keystore.(*watcher).loop(0xc00000e408)
        /Users/gashkov/dev/go-ethereum/accounts/keystore/watch.go:108 +0xbb4
created by github.com/ethereum/go-ethereum/accounts/keystore.(*watcher).start in goroutine 1
        /Users/gashkov/dev/go-ethereum/accounts/keystore/watch.go:56 +0x144

goroutine 10 [select]:
github.com/ethereum/go-ethereum/accounts/keystore.(*KeyStore).updater(0xc0000ce690)
        /Users/gashkov/dev/go-ethereum/accounts/keystore/keystore.go:209 +0xe4
created by github.com/ethereum/go-ethereum/accounts/keystore.(*KeyStore).Subscribe in goroutine 8
        /Users/gashkov/dev/go-ethereum/accounts/keystore/keystore.go:196 +0x238

goroutine 46 [select, 1 minutes]:
github.com/ethereum/go-ethereum/core/txpool/legacypool.(*LegacyPool).scheduleReorgLoop(0xc0003b9c00)
        /Users/gashkov/dev/go-ethereum/core/txpool/legacypool/legacypool.go:1215 +0x53c
created by github.com/ethereum/go-ethereum/core/txpool/legacypool.(*LegacyPool).Init in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/txpool/legacypool/legacypool.go:315 +0x430

goroutine 2032 [select, 1 minutes]:
github.com/ethereum/go-ethereum/core/txpool.(*TxPool).loop(0xc000513e60, 0xc000420780, {0x106f5ec18, 0xc00003e800})
        /Users/gashkov/dev/go-ethereum/core/txpool/txpool.go:243 +0x70c
created by github.com/ethereum/go-ethereum/core/txpool.New in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/txpool/txpool.go:103 +0x598

goroutine 4354 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth/fetcher.(*BlockFetcher).loop(0xc0002ca2a0)
        /Users/gashkov/dev/go-ethereum/eth/fetcher/block_fetcher.go:380 +0xb60
created by github.com/ethereum/go-ethereum/eth/fetcher.(*BlockFetcher).Start in goroutine 4336
        /Users/gashkov/dev/go-ethereum/eth/fetcher/block_fetcher.go:232 +0xb8

goroutine 4283 [select, 1 minutes]:
github.com/syndtr/goleveldb/leveldb.(*DB).tCompaction(0xc0001388c0)
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/syndtr/goleveldb@v1.0.1-0.20210819022825-2ae1ddf74ef7/leveldb/db_compaction.go:836 +0x5f0
created by github.com/syndtr/goleveldb/leveldb.openDB in goroutine 1
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/syndtr/goleveldb@v1.0.1-0.20210819022825-2ae1ddf74ef7/leveldb/db.go:155 +0xbac

goroutine 4276 [select, 1 minutes]:
github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1(0xc0004ea780)
        /Users/gashkov/dev/go-ethereum/event/multisub.go:43 +0x444
github.com/ethereum/go-ethereum/event.NewSubscription.func1()
        /Users/gashkov/dev/go-ethereum/event/subscription.go:53 +0x120
created by github.com/ethereum/go-ethereum/event.NewSubscription in goroutine 1
        /Users/gashkov/dev/go-ethereum/event/subscription.go:51 +0x258

goroutine 4277 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth/filters.(*EventSystem).eventLoop(0xc0000cbd90)
        /Users/gashkov/dev/go-ethereum/eth/filters/filter_system.go:563 +0x5a4
created by github.com/ethereum/go-ethereum/eth/filters.NewEventSystem in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/filters/filter_system.go:249 +0xa70

goroutine 4284 [select, 1 minutes]:
github.com/syndtr/goleveldb/leveldb.(*DB).mCompaction(0xc0001388c0)
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/syndtr/goleveldb@v1.0.1-0.20210819022825-2ae1ddf74ef7/leveldb/db_compaction.go:773 +0x200
created by github.com/syndtr/goleveldb/leveldb.openDB in goroutine 1
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/syndtr/goleveldb@v1.0.1-0.20210819022825-2ae1ddf74ef7/leveldb/db.go:156 +0xc3c

goroutine 4275 [select]:
github.com/ethereum/go-ethereum/eth/downloader.(*DownloaderAPI).eventLoop(0xc0005ea4e0)
        /Users/gashkov/dev/go-ethereum/eth/downloader/api.go:90 +0x41c
created by github.com/ethereum/go-ethereum/eth/downloader.NewDownloaderAPI in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/downloader/api.go:53 +0x2c0

goroutine 4286 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/p2p/enode.(*FairMix).nextFromAny(0xc0008bba40)
        /Users/gashkov/dev/go-ethereum/p2p/enode/iter.go:248 +0x60
github.com/ethereum/go-ethereum/p2p/enode.(*FairMix).Next(0xc0008bba40)
        /Users/gashkov/dev/go-ethereum/p2p/enode/iter.go:209 +0x428
github.com/ethereum/go-ethereum/p2p.(*dialScheduler).readNodes(0xc0003682c0, {0x106f628b0, 0xc0008bba40})
        /Users/gashkov/dev/go-ethereum/p2p/dial.go:321 +0x10c
created by github.com/ethereum/go-ethereum/p2p.newDialScheduler in goroutine 1
        /Users/gashkov/dev/go-ethereum/p2p/dial.go:180 +0x898

goroutine 4288 [select, 1 minutes]:
github.com/ethereum/go-ethereum/p2p.(*Server).run(0xc0001ecb00)
        /Users/gashkov/dev/go-ethereum/p2p/server.go:723 +0x8dc
created by github.com/ethereum/go-ethereum/p2p.(*Server).Start in goroutine 1
        /Users/gashkov/dev/go-ethereum/p2p/server.go:502 +0xa64

goroutine 4278 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/eth/filters.(*FilterAPI).timeoutLoop(0xc0005ea870, 0x45d964b800)
        /Users/gashkov/dev/go-ethereum/eth/filters/api.go:89 +0x13c
created by github.com/ethereum/go-ethereum/eth/filters.NewFilterAPI in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/filters/api.go:77 +0x2fc

goroutine 4285 [select, 1 minutes]:
github.com/ethereum/go-ethereum/p2p.(*Server).portMappingLoop(0xc0001ecb00)
        /Users/gashkov/dev/go-ethereum/p2p/server_nat.go:118 +0x700
created by github.com/ethereum/go-ethereum/p2p.(*Server).setupPortMapping in goroutine 1
        /Users/gashkov/dev/go-ethereum/p2p/server_nat.go:70 +0x348

goroutine 4282 [select]:
github.com/syndtr/goleveldb/leveldb.(*DB).mpoolDrain(0xc0001388c0)
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/syndtr/goleveldb@v1.0.1-0.20210819022825-2ae1ddf74ef7/leveldb/db_state.go:101 +0x108
created by github.com/syndtr/goleveldb/leveldb.openDB in goroutine 1
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/syndtr/goleveldb@v1.0.1-0.20210819022825-2ae1ddf74ef7/leveldb/db.go:149 +0xae4

goroutine 41 [select, 1 minutes]:
github.com/ethereum/go-ethereum/core.(*ChainIndexer).updateLoop(0xc0000ce870)
        /Users/gashkov/dev/go-ethereum/core/chain_indexer.go:312 +0x114
created by github.com/ethereum/go-ethereum/core.NewChainIndexer in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/chain_indexer.go:120 +0x65c

goroutine 4287 [select, 1 minutes]:
github.com/ethereum/go-ethereum/p2p.(*dialScheduler).loop(0xc0003682c0, {0x106f628b0, 0xc0008bba40})
        /Users/gashkov/dev/go-ethereum/p2p/dial.go:242 +0x31c
created by github.com/ethereum/go-ethereum/p2p.newDialScheduler in goroutine 1
        /Users/gashkov/dev/go-ethereum/p2p/dial.go:181 +0x99c

goroutine 4280 [select, 1 minutes]:
github.com/syndtr/goleveldb/leveldb.(*session).refLoop(0xc0000cec30)
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/syndtr/goleveldb@v1.0.1-0.20210819022825-2ae1ddf74ef7/leveldb/session_util.go:189 +0xce0
created by github.com/syndtr/goleveldb/leveldb.newSession in goroutine 1
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/syndtr/goleveldb@v1.0.1-0.20210819022825-2ae1ddf74ef7/leveldb/session.go:93 +0x5fc

goroutine 4279 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/core/txpool.(*TxPool).Sync(0xc000513e60)
        /Users/gashkov/dev/go-ethereum/core/txpool/txpool.go:478 +0x148
github.com/ethereum/go-ethereum/eth/catalyst.(*ConsensusAPI).forkchoiceUpdated(0xc000178be0, {{0xc9, 0x10, 0x26, 0x77, 0xa1, 0x3, 0x24, 0x7, 0xab, ...}, ...}, ...)
        /Users/gashkov/dev/go-ethereum/eth/catalyst/api.go:397 +0x28d0
github.com/ethereum/go-ethereum/eth/catalyst.(*SimulatedBeacon).sealBlock(0xc0006626e0, {0x108c0da00, 0x0, 0x0}, 0x661190bc)
        /Users/gashkov/dev/go-ethereum/eth/catalyst/simulated_beacon.go:159 +0x4f4
github.com/ethereum/go-ethereum/eth/catalyst.(*SimulatedBeacon).Commit(0xc0006626e0)
        /Users/gashkov/dev/go-ethereum/eth/catalyst/simulated_beacon.go:249 +0xc4
github.com/ethereum/go-ethereum/eth/catalyst.(*api).loop(0xc0001b6980)
        /Users/gashkov/dev/go-ethereum/eth/catalyst/simulated_beacon_api.go:50 +0x2d8
created by github.com/ethereum/go-ethereum/eth/catalyst.RegisterSimulatedBeaconAPIs in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/catalyst/simulated_beacon.go:294 +0x14c

goroutine 4281 [select, 1 minutes]:
github.com/syndtr/goleveldb/leveldb.(*DB).compactionError(0xc0001388c0)
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/syndtr/goleveldb@v1.0.1-0.20210819022825-2ae1ddf74ef7/leveldb/db_compaction.go:91 +0x114
created by github.com/syndtr/goleveldb/leveldb.openDB in goroutine 1
        /Users/gashkov/.asdf/installs/golang/1.21.0/packages/pkg/mod/github.com/syndtr/goleveldb@v1.0.1-0.20210819022825-2ae1ddf74ef7/leveldb/db.go:148 +0xa54

goroutine 4268 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers.func1()
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:50 +0x114
created by github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:48 +0xf4

goroutine 45 [select, 1 minutes]:
github.com/ethereum/go-ethereum/core.(*ChainIndexer).eventLoop(0xc0000ce870, 0xc000283900, 0xc000513d40, {0x106f5f690, 0xc000eafc50})
        /Users/gashkov/dev/go-ethereum/core/chain_indexer.go:212 +0x27c
created by github.com/ethereum/go-ethereum/core.(*ChainIndexer).Start in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/chain_indexer.go:153 +0x260

goroutine 42 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/core/state/snapshot.(*diskLayer).generate(0xc000490870, 0xc0000303c0)
        /Users/gashkov/dev/go-ethereum/core/state/snapshot/generate.go:722 +0xc3c
created by github.com/ethereum/go-ethereum/core/state/snapshot.generateSnapshot in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/state/snapshot/generate.go:80 +0x7e0

goroutine 44 [select]:
github.com/ethereum/go-ethereum/core.(*txIndexer).loop(0xc00051b980, 0xc00003e800)
        /Users/gashkov/dev/go-ethereum/core/txindexer.go:149 +0x710
created by github.com/ethereum/go-ethereum/core.newTxIndexer in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/txindexer.go:63 +0x330

goroutine 4272 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers.func1()
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:50 +0x114
created by github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:48 +0xf4

goroutine 43 [select]:
github.com/ethereum/go-ethereum/core.(*BlockChain).updateFutureBlocks(0xc00003e800)
        /Users/gashkov/dev/go-ethereum/core/blockchain.go:2342 +0x278
created by github.com/ethereum/go-ethereum/core.NewBlockChain in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/blockchain.go:455 +0x2e44

goroutine 47 [select]:
github.com/ethereum/go-ethereum/core/txpool/legacypool.(*LegacyPool).loop(0xc0003b9c00)
        /Users/gashkov/dev/go-ethereum/core/txpool/legacypool/legacypool.go:352 +0x4f0
created by github.com/ethereum/go-ethereum/core/txpool/legacypool.(*LegacyPool).Init in goroutine 1
        /Users/gashkov/dev/go-ethereum/core/txpool/legacypool/legacypool.go:327 +0x8e4

goroutine 4274 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/eth/gasprice.NewOracle.func1()
        /Users/gashkov/dev/go-ethereum/eth/gasprice/gasprice.go:123 +0x6c
created by github.com/ethereum/go-ethereum/eth/gasprice.NewOracle in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/gasprice/gasprice.go:121 +0x1138

goroutine 2033 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth/downloader.(*skeleton).startup(0xc0002ca1c0)
        /Users/gashkov/dev/go-ethereum/eth/downloader/skeleton.go:258 +0x1e4
created by github.com/ethereum/go-ethereum/eth/downloader.newSkeleton in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/downloader/skeleton.go:241 +0x470

goroutine 4355 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth/fetcher.(*TxFetcher).loop(0xc0009300b0)
        /Users/gashkov/dev/go-ethereum/eth/fetcher/tx_fetcher.go:409 +0x24c
created by github.com/ethereum/go-ethereum/eth/fetcher.(*TxFetcher).Start in goroutine 4336
        /Users/gashkov/dev/go-ethereum/eth/fetcher/tx_fetcher.go:391 +0xb8

goroutine 48 [select, 1 minutes]:
github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1.2({0x106f5f6e0, 0xc0001122c0})
        /Users/gashkov/dev/go-ethereum/event/multisub.go:33 +0xe4
created by github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1 in goroutine 4259
        /Users/gashkov/dev/go-ethereum/event/multisub.go:32 +0x398

goroutine 4624 [chan receive]:
github.com/ethereum/go-ethereum/rpc.(*Server).ServeCodec(0xc000108370, {0x106f69cb0, 0xc000a08460}, 0x0)
        /Users/gashkov/dev/go-ethereum/rpc/server.go:117 +0x3e8
created by github.com/ethereum/go-ethereum/rpc.(*Server).ServeListener in goroutine 4265
        /Users/gashkov/dev/go-ethereum/rpc/ipc.go:38 +0x508

goroutine 4333 [select, 1 minutes]:
github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1(0xc0005e93e0)
        /Users/gashkov/dev/go-ethereum/event/multisub.go:43 +0x444
github.com/ethereum/go-ethereum/event.NewSubscription.func1()
        /Users/gashkov/dev/go-ethereum/event/subscription.go:53 +0x120
created by github.com/ethereum/go-ethereum/event.NewSubscription in goroutine 1
        /Users/gashkov/dev/go-ethereum/event/subscription.go:51 +0x258

goroutine 4269 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers.func1()
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:50 +0x114
created by github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:48 +0xf4

goroutine 4369 [select, 1 minutes]:
reflect.rselect({0xc00001d148, 0x2, 0x99?})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/runtime/select.go:589 +0x2d0
reflect.Select({0xc000000b40, 0x2, 0x5})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/reflect/value.go:3104 +0xa00
github.com/ethereum/go-ethereum/event.(*Feed).Send(0xc0003b9c90, {0x106d49200, 0xc000b78078})
        /Users/gashkov/dev/go-ethereum/event/feed.go:160 +0x758
github.com/ethereum/go-ethereum/core/txpool/legacypool.(*LegacyPool).runReorg(0xc0003b9c00, 0xc0005e8fc0, 0xc000488ed0, 0xc000458ce0, 0xc00089d290)
        /Users/gashkov/dev/go-ethereum/core/txpool/legacypool/legacypool.go:1336 +0xbf0
created by github.com/ethereum/go-ethereum/core/txpool/legacypool.(*LegacyPool).scheduleReorgLoop in goroutine 46
        /Users/gashkov/dev/go-ethereum/core/txpool/legacypool/legacypool.go:1205 +0x37c

goroutine 67 [select, 1 minutes]:
github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1(0xc0005e8180)
        /Users/gashkov/dev/go-ethereum/event/multisub.go:43 +0x444
github.com/ethereum/go-ethereum/event.NewSubscription.func1()
        /Users/gashkov/dev/go-ethereum/event/subscription.go:53 +0x120
created by github.com/ethereum/go-ethereum/event.NewSubscription in goroutine 4279
        /Users/gashkov/dev/go-ethereum/event/subscription.go:51 +0x258

goroutine 4267 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth/protocols/eth.StartENRUpdater.func1()
        /Users/gashkov/dev/go-ethereum/eth/protocols/eth/discovery.go:48 +0x1c8
created by github.com/ethereum/go-ethereum/eth/protocols/eth.StartENRUpdater in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/protocols/eth/discovery.go:45 +0x218

goroutine 4266 [IO wait, 1 minutes]:
internal/poll.runtime_pollWait(0xc0008832a8?, 0x72)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/runtime/netpoll.go:343 +0x44
internal/poll.(*pollDesc).wait(0xc0009869a0, 0x72, 0x0)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/internal/poll/fd_poll_runtime.go:84 +0xc0
internal/poll.(*pollDesc).waitRead(0xc0009869a0, 0x0)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/internal/poll/fd_poll_runtime.go:89 +0x54
internal/poll.(*FD).Accept(0xc000986980)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/internal/poll/fd_unix.go:611 +0x46c
net.(*netFD).accept(0xc000986980)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/net/fd_unix.go:172 +0x58
net.(*TCPListener).accept(0xc000271140)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/net/tcpsock_posix.go:152 +0x5c
net.(*TCPListener).Accept(0xc000271140)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/net/tcpsock.go:315 +0x60
net/http.(*Server).Serve(0xc000534000, {0x106f626d0, 0xc000271140})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/net/http/server.go:3056 +0x654
created by github.com/ethereum/go-ethereum/node.(*httpServer).start in goroutine 1
        /Users/gashkov/dev/go-ethereum/node/rpcstack.go:160 +0x75c

goroutine 54 [select, 1 minutes]:
github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1.2({0x106f5f6e0, 0xc000031c00})
        /Users/gashkov/dev/go-ethereum/event/multisub.go:33 +0xe4
created by github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1 in goroutine 4333
        /Users/gashkov/dev/go-ethereum/event/multisub.go:32 +0x398

goroutine 4264 [select, 1 minutes]:
github.com/ethereum/go-ethereum/miner.(*Miner).update(0xc0005786c0)
        /Users/gashkov/dev/go-ethereum/miner/miner.go:117 +0x470
created by github.com/ethereum/go-ethereum/miner.New in goroutine 1
        /Users/gashkov/dev/go-ethereum/miner/miner.go:95 +0x4a4

goroutine 4270 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers.func1()
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:50 +0x114
created by github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:48 +0xf4

goroutine 4260 [select, 1 minutes]:
github.com/ethereum/go-ethereum/miner.(*worker).mainLoop(0xc00038f200)
        /Users/gashkov/dev/go-ethereum/miner/worker.go:533 +0x564
created by github.com/ethereum/go-ethereum/miner.newWorker in goroutine 1
        /Users/gashkov/dev/go-ethereum/miner/worker.go:294 +0x128c

goroutine 4271 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers.func1()
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:50 +0x114
created by github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:48 +0xf4

goroutine 4265 [IO wait]:
internal/poll.runtime_pollWait(0xc000b9b5b8?, 0x72)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/runtime/netpoll.go:343 +0x44
internal/poll.(*pollDesc).wait(0xc000986520, 0x72, 0x0)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/internal/poll/fd_poll_runtime.go:84 +0xc0
internal/poll.(*pollDesc).waitRead(0xc000986520, 0x0)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/internal/poll/fd_poll_runtime.go:89 +0x54
internal/poll.(*FD).Accept(0xc000986500)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/internal/poll/fd_unix.go:611 +0x46c
net.(*netFD).accept(0xc000986500)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/net/fd_unix.go:172 +0x58
net.(*UnixListener).accept(0xc0005eaa80)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/net/unixsock_posix.go:172 +0x5c
net.(*UnixListener).Accept(0xc0005eaa80)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/net/unixsock.go:260 +0x60
github.com/ethereum/go-ethereum/rpc.(*Server).ServeListener(0xc000108370, {0x106f62700, 0xc0005eaa80})
        /Users/gashkov/dev/go-ethereum/rpc/ipc.go:30 +0x60
created by github.com/ethereum/go-ethereum/rpc.StartIPCEndpoint in goroutine 1
        /Users/gashkov/dev/go-ethereum/rpc/endpoints.go:50 +0x960

goroutine 4298 [select, 1 minutes]:
github.com/ethereum/go-ethereum/rpc.(*Client).dispatch(0xc0001681b0, {0x106f69cb0, 0xc0000c1a40})
        /Users/gashkov/dev/go-ethereum/rpc/client.go:641 +0x508
created by github.com/ethereum/go-ethereum/rpc.initClient in goroutine 4339
        /Users/gashkov/dev/go-ethereum/rpc/client.go:269 +0x898

goroutine 49 [select, 1 minutes]:
github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1.2({0x106f5f6e0, 0xc0001123c0})
        /Users/gashkov/dev/go-ethereum/event/multisub.go:33 +0xe4
created by github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1 in goroutine 4259
        /Users/gashkov/dev/go-ethereum/event/multisub.go:32 +0x398

goroutine 55 [select, 1 minutes]:
github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1.2({0x106f5f6e0, 0xc000031c40})
        /Users/gashkov/dev/go-ethereum/event/multisub.go:33 +0xe4
created by github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1 in goroutine 4333
        /Users/gashkov/dev/go-ethereum/event/multisub.go:32 +0x398

goroutine 4290 [select, 1 minutes]:
github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1.2({0x106f5f6e0, 0xc00011a180})
        /Users/gashkov/dev/go-ethereum/event/multisub.go:33 +0xe4
created by github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1 in goroutine 4276
        /Users/gashkov/dev/go-ethereum/event/multisub.go:32 +0x398

goroutine 4323 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers.func1()
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:50 +0x114
created by github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:48 +0xf4

goroutine 4261 [select]:
github.com/ethereum/go-ethereum/miner.(*worker).newWorkLoop(0xc00038f200, 0x77359400)
        /Users/gashkov/dev/go-ethereum/miner/worker.go:460 +0x620
created by github.com/ethereum/go-ethereum/miner.newWorker in goroutine 1
        /Users/gashkov/dev/go-ethereum/miner/worker.go:295 +0x134c

goroutine 4262 [select, 1 minutes]:
github.com/ethereum/go-ethereum/miner.(*worker).resultLoop(0xc00038f200)
        /Users/gashkov/dev/go-ethereum/miner/worker.go:653 +0x1c4
created by github.com/ethereum/go-ethereum/miner.newWorker in goroutine 1
        /Users/gashkov/dev/go-ethereum/miner/worker.go:296 +0x13dc

goroutine 4325 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers.func1()
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:50 +0x114
created by github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:48 +0xf4

goroutine 4273 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers.func1()
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:50 +0x114
created by github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:48 +0xf4

goroutine 4625 [select]:
github.com/ethereum/go-ethereum/rpc.(*Client).dispatch(0xc000490900, {0x106f69cb0, 0xc000a08460})
        /Users/gashkov/dev/go-ethereum/rpc/client.go:641 +0x508
created by github.com/ethereum/go-ethereum/rpc.initClient in goroutine 4624
        /Users/gashkov/dev/go-ethereum/rpc/client.go:269 +0x898

goroutine 4259 [select, 1 minutes]:
github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1(0xc000049080)
        /Users/gashkov/dev/go-ethereum/event/multisub.go:43 +0x444
github.com/ethereum/go-ethereum/event.NewSubscription.func1()
        /Users/gashkov/dev/go-ethereum/event/subscription.go:53 +0x120
created by github.com/ethereum/go-ethereum/event.NewSubscription in goroutine 1
        /Users/gashkov/dev/go-ethereum/event/subscription.go:51 +0x258

goroutine 4674 [IO wait]:
internal/poll.runtime_pollWait(0xc00056b2c8?, 0x72)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/runtime/netpoll.go:343 +0x44
internal/poll.(*pollDesc).wait(0xc00087e220, 0x72, 0x0)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/internal/poll/fd_poll_runtime.go:84 +0xc0
internal/poll.(*pollDesc).waitRead(0xc00087e220, 0x0)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/internal/poll/fd_poll_runtime.go:89 +0x54
internal/poll.(*FD).Read(0xc00087e200, {0xc000340001, 0x5ff, 0x5ff})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/internal/poll/fd_unix.go:164 +0x420
net.(*netFD).Read(0xc00087e200, {0xc000340001, 0x5ff, 0x5ff})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/net/fd_posix.go:55 +0x70
net.(*conn).Read(0xc000800040, {0xc000340001, 0x5ff, 0x5ff})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/net/net.go:179 +0x9c
encoding/json.(*Decoder).refill(0xc000c26280)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/encoding/json/stream.go:165 +0x4e4
encoding/json.(*Decoder).readValue(0xc000c26280)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/encoding/json/stream.go:140 +0x5fc
encoding/json.(*Decoder).Decode(0xc000c26280, {0x106d08860, 0xc0009ae120})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/encoding/json/stream.go:63 +0x10c
github.com/ethereum/go-ethereum/rpc.(*jsonCodec).readBatch(0xc000a08460)
        /Users/gashkov/dev/go-ethereum/rpc/json.go:236 +0xd4
github.com/ethereum/go-ethereum/rpc.(*Client).read(0xc000490900, {0x106f69cb0, 0xc000a08460})
        /Users/gashkov/dev/go-ethereum/rpc/client.go:714 +0x60
created by github.com/ethereum/go-ethereum/rpc.(*Client).dispatch in goroutine 4625
        /Users/gashkov/dev/go-ethereum/rpc/client.go:638 +0x2d4

goroutine 4258 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth/downloader.(*Downloader).stateFetcher(0xc000efcea0)
        /Users/gashkov/dev/go-ethereum/eth/downloader/statesync.go:47 +0xfc
created by github.com/ethereum/go-ethereum/eth/downloader.New in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/downloader/downloader.go:240 +0x7dc

goroutine 79 [select, 1 minutes]:
github.com/ethereum/go-ethereum/core/state.(*subfetcher).loop(0xc0009b4300)
        /Users/gashkov/dev/go-ethereum/core/state/trie_prefetcher.go:319 +0x9f4
created by github.com/ethereum/go-ethereum/core/state.newSubfetcher in goroutine 4260
        /Users/gashkov/dev/go-ethereum/core/state/trie_prefetcher.go:246 +0x46c

goroutine 4383 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/core/txpool/legacypool.(*LegacyPool).Reset(0xc0003b9c00, 0xc000b3e000, 0xc000273900)
        /Users/gashkov/dev/go-ethereum/core/txpool/legacypool/legacypool.go:418 +0x70
github.com/ethereum/go-ethereum/core/txpool.(*TxPool).loop.func2(0xc000b3e000, 0xc000273900)
        /Users/gashkov/dev/go-ethereum/core/txpool/txpool.go:224 +0xfc
created by github.com/ethereum/go-ethereum/core/txpool.(*TxPool).loop in goroutine 2032
        /Users/gashkov/dev/go-ethereum/core/txpool/txpool.go:222 +0x5b8

goroutine 4337 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*handler).protoTracker(0xc00031ef70)
        /Users/gashkov/dev/go-ethereum/eth/handler.go:294 +0x1e8
created by github.com/ethereum/go-ethereum/eth.(*handler).Start in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/handler.go:534 +0x558

goroutine 4342 [select, 1 minutes]:
net.(*pipe).read(0xc000986a80, {0xc00024a200, 0x200, 0x200})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/net/pipe.go:159 +0x2f8
net.(*pipe).Read(0xc000986a80, {0xc00024a200, 0x200, 0x200})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/net/pipe.go:142 +0x70
encoding/json.(*Decoder).refill(0xc0000c52c0)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/encoding/json/stream.go:165 +0x4e4
encoding/json.(*Decoder).readValue(0xc0000c52c0)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/encoding/json/stream.go:140 +0x5fc
encoding/json.(*Decoder).Decode(0xc0000c52c0, {0x106d08860, 0xc000258a68})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/encoding/json/stream.go:63 +0x10c
github.com/ethereum/go-ethereum/rpc.(*jsonCodec).readBatch(0xc0000c1a90)
        /Users/gashkov/dev/go-ethereum/rpc/json.go:236 +0xd4
github.com/ethereum/go-ethereum/rpc.(*Client).read(0xc0004902d0, {0x106f69cb0, 0xc0000c1a90})
        /Users/gashkov/dev/go-ethereum/rpc/client.go:714 +0x60
created by github.com/ethereum/go-ethereum/rpc.(*Client).dispatch in goroutine 4340
        /Users/gashkov/dev/go-ethereum/rpc/client.go:638 +0x2d4

goroutine 4341 [chan receive, 1 minutes]:
main.startNode.func1()
        /Users/gashkov/dev/go-ethereum/cmd/geth/main.go:376 +0x3bc
created by main.startNode in goroutine 1
        /Users/gashkov/dev/go-ethereum/cmd/geth/main.go:368 +0x26c

goroutine 4340 [select, 1 minutes]:
github.com/ethereum/go-ethereum/rpc.(*Client).dispatch(0xc0004902d0, {0x106f69cb0, 0xc0000c1a90})
        /Users/gashkov/dev/go-ethereum/rpc/client.go:641 +0x508
created by github.com/ethereum/go-ethereum/rpc.initClient in goroutine 1
        /Users/gashkov/dev/go-ethereum/rpc/client.go:269 +0x898

goroutine 4339 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/rpc.(*Server).ServeCodec(0xc00029c7d0, {0x106f69cb0, 0xc0000c1a40}, 0x0)
        /Users/gashkov/dev/go-ethereum/rpc/server.go:117 +0x3e8
created by github.com/ethereum/go-ethereum/rpc.DialInProc.func1 in goroutine 1
        /Users/gashkov/dev/go-ethereum/rpc/inproc.go:30 +0x1d8

goroutine 4338 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/cmd/utils.StartNode.func1()
        /Users/gashkov/dev/go-ethereum/cmd/utils/cmd.go:120 +0x6c8
created by github.com/ethereum/go-ethereum/cmd/utils.StartNode in goroutine 1
        /Users/gashkov/dev/go-ethereum/cmd/utils/cmd.go:81 +0x270

goroutine 4334 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*handler).txBroadcastLoop(0xc00031ef70)
        /Users/gashkov/dev/go-ethereum/eth/handler.go:668 +0x1cc
created by github.com/ethereum/go-ethereum/eth.(*handler).Start in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/handler.go:521 +0x24c

goroutine 4336 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*chainSyncer).loop(0xc000112280)
        /Users/gashkov/dev/go-ethereum/eth/sync.go:108 +0x718
created by github.com/ethereum/go-ethereum/eth.(*handler).Start in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/handler.go:530 +0x4ac

goroutine 4335 [chan receive, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*handler).minedBroadcastLoop(0xc00031ef70)
        /Users/gashkov/dev/go-ethereum/eth/handler.go:656 +0x138
created by github.com/ethereum/go-ethereum/eth.(*handler).Start in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/handler.go:526 +0x3e8

goroutine 4332 [select, 1 minutes]:
github.com/ethereum/go-ethereum/internal/shutdowncheck.(*ShutdownTracker).Start.func1()
        /Users/gashkov/dev/go-ethereum/internal/shutdowncheck/shutdown_tracker.go:69 +0x1bc
created by github.com/ethereum/go-ethereum/internal/shutdowncheck.(*ShutdownTracker).Start in goroutine 1
        /Users/gashkov/dev/go-ethereum/internal/shutdowncheck/shutdown_tracker.go:65 +0xb0

goroutine 4331 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers.func1()
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:50 +0x114
created by github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:48 +0xf4

goroutine 4330 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers.func1()
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:50 +0x114
created by github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:48 +0xf4

goroutine 4326 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers.func1()
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:50 +0x114
created by github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:48 +0xf4

goroutine 4329 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers.func1()
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:50 +0x114
created by github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:48 +0xf4

goroutine 4328 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers.func1()
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:50 +0x114
created by github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:48 +0xf4

goroutine 4327 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers.func1()
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:50 +0x114
created by github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:48 +0xf4

goroutine 4291 [select, 1 minutes]:
github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1.2({0x106f5f6e0, 0xc00011a1c0})
        /Users/gashkov/dev/go-ethereum/event/multisub.go:33 +0xe4
created by github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1 in goroutine 4276
        /Users/gashkov/dev/go-ethereum/event/multisub.go:32 +0x398

goroutine 4299 [select, 1 minutes]:
net.(*pipe).read(0xc000986a00, {0xc0001fe600, 0x200, 0x200})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/net/pipe.go:159 +0x2f8
net.(*pipe).Read(0xc000986a00, {0xc0001fe600, 0x200, 0x200})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/net/pipe.go:142 +0x70
encoding/json.(*Decoder).refill(0xc0000c4f00)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/encoding/json/stream.go:165 +0x4e4
encoding/json.(*Decoder).readValue(0xc0000c4f00)
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/encoding/json/stream.go:140 +0x5fc
encoding/json.(*Decoder).Decode(0xc0000c4f00, {0x106d08860, 0xc0008b8060})
        /Users/gashkov/.asdf/installs/golang/1.21.0/go/src/encoding/json/stream.go:63 +0x10c
github.com/ethereum/go-ethereum/rpc.(*jsonCodec).readBatch(0xc0000c1a40)
        /Users/gashkov/dev/go-ethereum/rpc/json.go:236 +0xd4
github.com/ethereum/go-ethereum/rpc.(*Client).read(0xc0001681b0, {0x106f69cb0, 0xc0000c1a40})
        /Users/gashkov/dev/go-ethereum/rpc/client.go:714 +0x60
created by github.com/ethereum/go-ethereum/rpc.(*Client).dispatch in goroutine 4298
        /Users/gashkov/dev/go-ethereum/rpc/client.go:638 +0x2d4

goroutine 4263 [select, 1 minutes]:
github.com/ethereum/go-ethereum/miner.(*worker).taskLoop(0xc00038f200)
        /Users/gashkov/dev/go-ethereum/miner/worker.go:614 +0x250
created by github.com/ethereum/go-ethereum/miner.newWorker in goroutine 1
        /Users/gashkov/dev/go-ethereum/miner/worker.go:297 +0x146c

goroutine 4324 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers.func1()
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:50 +0x114
created by github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:48 +0xf4

goroutine 69 [select, 1 minutes]:
github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1.2({0x106f5f6e0, 0xc0000300c0})
        /Users/gashkov/dev/go-ethereum/event/multisub.go:33 +0xe4
created by github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1 in goroutine 67
        /Users/gashkov/dev/go-ethereum/event/multisub.go:32 +0x398

goroutine 68 [select, 1 minutes]:
github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1.2({0x106f5f6e0, 0xc000030080})
        /Users/gashkov/dev/go-ethereum/event/multisub.go:33 +0xe4
created by github.com/ethereum/go-ethereum/event.JoinSubscriptions.func1 in goroutine 67
        /Users/gashkov/dev/go-ethereum/event/multisub.go:32 +0x398

goroutine 4322 [select, 1 minutes]:
github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers.func1()
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:50 +0x114
created by github.com/ethereum/go-ethereum/eth.(*Ethereum).startBloomHandlers in goroutine 1
        /Users/gashkov/dev/go-ethereum/eth/bloombits.go:48 +0xf4
```
</details>
