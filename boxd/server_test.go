// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package boxd

import (
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	config "github.com/BOXFoundation/boxd/config"
	"github.com/jbenet/goprocess"
)

const (
	cfgJSON = `{
  "Workspace": ".devconfig/ws1",
  "Network": "mainnet",
  "Log": {
    "out": {
      "name": "stderr",
      "options": null
    },
    "level": "debug",
    "hooks": [
      {
        "name": "filewithformatter",
        "options": {
          "filename": "box.log",
          "level": 4,
          "maxlines": 100000,
          "rotate": true
        }
      }
    ],
    "formatter": {
      "name": "text",
      "options": null
    }
  },
  "P2p": {
    "Magic": 0,
    "KeyPath": "peer.key",
    "Port": 19199,
    "Address": "0.0.0.0",
    "Seeds": null,
    "Bucketsize": 16,
    "Latency": 10,
    "AddPeers": null
  },
  "RPC": {
    "Enabled": true,
    "Address": "127.0.0.1",
    "Port": 19191,
    "HTTP": {
      "Address": "127.0.0.1",
      "Port": 19190
    }
  },
  "Database": {
    "Name": "rocksdb",
    "Path": "",
    "Options": null
  },
  "Dpos": {
    "Index": 0,
    "Pubkey": "e34cce70c86373273efcc54ce7d2a491bb4a0e84"
  }
}`
)

type testCfg struct {
	ws          string
	p2pPort     uint32
	rpcPort     int
	rpcHTTPPort int
	dposIdx     int
	dposPubKey  string
	seeds       []string
}

func newTestCfg(ws string, p2pPort uint32, rpcPort, rpcHTTPPort int,
	seeds []string, dposIdx int, dposPubKey string) *testCfg {
	return &testCfg{
		ws:          ws,
		p2pPort:     p2pPort,
		rpcPort:     rpcPort,
		rpcHTTPPort: rpcHTTPPort,
		dposIdx:     dposIdx,
		dposPubKey:  dposPubKey,
		seeds:       seeds,
	}
}

func genCfg(tc *testCfg) *config.Config {
	cfg := &config.Config{}
	json.Unmarshal([]byte(cfgJSON), cfg)
	cfg.Workspace = tc.ws
	cfg.P2p.Port = tc.p2pPort
	cfg.RPC.Port = tc.rpcPort
	cfg.RPC.HTTP.Port = tc.rpcHTTPPort
	cfg.Dpos.Index = tc.dposIdx
	cfg.Dpos.Pubkey = tc.dposPubKey
	cfg.P2p.Seeds = tc.seeds

	return cfg
}

func prepareTestSvr(cfg *config.Config, d time.Duration) *Server {
	// set a time out process
	proc := goprocess.WithSignals(os.Interrupt)
	go func() {
		timer := time.NewTimer(d)
		select {
		case <-timer.C:
			proc.Close()
		}
	}()
	// new server and start
	svr := NewServer(cfg)
	svr.proc = proc
	svr.Prepare()
	return svr
}

func cleanWs(ws string) error {
	if err := os.RemoveAll(ws + "/database"); err != nil {
		return err
	}
	if err := os.RemoveAll(ws + "/logs"); err != nil {
		return err
	}
	return nil
}

func TestSyncManager(t *testing.T) {
	ws1, ws2 := ".devconfig/ws1", ".devconfig/ws2"
	cleanWs(ws1)
	cleanWs(ws2)
	defer cleanWs(ws1)
	defer cleanWs(ws2)

	var wg sync.WaitGroup
	wg.Add(1)

	// server1
	ws1PubKey := "e34cce70c86373273efcc54ce7d2a491bb4a0e84"
	tc1 := newTestCfg(ws1, 10019, 10011, 10010, nil, 0, ws1PubKey)
	cfg1 := genCfg(tc1)
	svr1 := prepareTestSvr(cfg1, 40*time.Second)
	go func() {
		svr1.Run()
		wg.Done()
	}()

	time.Sleep(20 * time.Second)
	// server2
	ws2PubKey := "0ef030107fd26e0b6bf40512bca2ceb1dd80adaa"
	seeds := []string{
		"/ip4/127.0.0.1/tcp/10019/p2p/12D3KooWFQ2naj8XZUVyGhFzBTEMrMc6emiCEDKLjaJMsK7p8Cza",
	}
	tc2 := newTestCfg(ws2, 10029, 10021, 10020, seeds, 5, ws2PubKey)
	cfg2 := genCfg(tc2)
	svr2 := prepareTestSvr(cfg2, 10*time.Second)
	svr2.Run()

	verifyChainTail(t, svr1, svr2)

	svr2 = prepareTestSvr(cfg2, 10*time.Second)
	svr2.Run()

	wg.Wait()

	verifyChainTail(t, svr1, svr2)
}

func verifyChainTail(t *testing.T, svr1, svr2 *Server) {
	tail1, tail2 := svr1.blockChain.TailBlock(), svr2.blockChain.TailBlock()
	h1, h2 := tail1.Height, tail2.Height
	hash1, hash2 := tail1.BlockHash(), tail2.BlockHash()
	if h1 != h2 || *hash1 != *hash2 {
		t.Fatalf("tail height and hash for server1 is %d, %v, for server2 is %d, %v",
			h1, hash1, h2, hash2)
	}
}
