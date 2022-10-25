package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	mh "github.com/multiformats/go-multihash"
	"github.com/urfave/cli/v2"
)

var log = logging.Logger("main")

var (
	flagCount         = "count"
	flagDuration      = "duration"
	flagAutoTest      = "auto"
	flagTestCIDsCount = "num-test-cids"
	flagLog           = "log"

	app = &cli.App{
		Name:                 "dht-tester",
		Usage:                "test libp2p nodes running go-libp2p-kad-dht",
		Action:               run,
		EnableBashCompletion: true,
		Suggest:              true,
		Flags: []cli.Flag{
			&cli.UintFlag{
				Name:  flagCount,
				Usage: "number of nodes to run",
				Value: 10,
			},
			&cli.UintFlag{
				Name:  flagDuration,
				Usage: "length of time to run simulation in seconds",
				Value: 600,
			},
			&cli.BoolFlag{
				Name:  flagAutoTest,
				Usage: "automatically provide and look up test CIDs",
				Value: false,
			},
			&cli.IntFlag{
				Name:  flagTestCIDsCount,
				Usage: "number of test CIDs to generate",
				Value: 20,
			},
			&cli.StringFlag{
				Name:  flagLog,
				Usage: "log level: one of [error|warn|info|debug]",
				Value: "info",
			},
		},
	}
)

// test CIDs generated at startup
var cids []cid.Cid

// list of all nodes's AddrInfo, used as bootnodes
var bootnodes []peer.AddrInfo

func bootstrapPeersFunc() []peer.AddrInfo {
	if len(bootnodes) == 0 {
		return bootnodes
	}

	bns := []peer.AddrInfo{}
	for i := 0; i < numPeers; i++ {
		randIdx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(bootnodes))))
		bns = append(bns, bootnodes[randIdx.Int64()])
	}
	return bootnodes
}

func main() {
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func setLogLevelsFromContext(c *cli.Context) error {
	const (
		levelError = "error"
		levelWarn  = "warn"
		levelInfo  = "info"
		levelDebug = "debug"
	)

	level := c.String(flagLog)
	switch level {
	case levelError, levelWarn, levelInfo, levelDebug:
	default:
		return fmt.Errorf("invalid log level %q", level)
	}

	_ = logging.SetLogLevel("main", level)
	_ = logging.SetLogLevel("dht", level)
	_ = logging.SetLogLevel("providers", level)
	return nil
}

func run(c *cli.Context) error {
	err := setLogLevelsFromContext(c)
	if err != nil {
		return err
	}

	cids = getTestCIDs(c.Int(flagTestCIDsCount))

	const basePort = 6000

	hosts := []*host{}

	count := int(c.Uint(flagCount))
	autoTest := c.Bool(flagAutoTest)

	for i := 0; i < count; i++ {
		log.Infof("starting node %d", i)
		cfg := &config{
			Ctx:      context.Background(),
			Port:     uint16(basePort + i),
			Index:    i,
			AutoTest: autoTest,
		}

		h, err := newHost(cfg)
		if err != nil {
			return err
		}

		bootnodes = append(bootnodes, h.addrInfo())
		hosts = append(hosts, h)
	}

	time.Sleep(time.Millisecond * 300)

	for i, h := range hosts {
		err := h.start()
		if err != nil {
			return err
		}

		log.Infof("node %d started: %s", i, h.addrInfo())
	}

	server, err := NewServer(hosts)
	if err != nil {
		return err
	}

	err = server.Start()
	if err != nil {
		return err
	}

	duration, err := time.ParseDuration(fmt.Sprintf("%ds", c.Uint(flagDuration)))
	if err != nil {
		return err
	}
	<-time.After(duration)

	for _, h := range hosts {
		err := h.stop()
		if err != nil {
			return err
		}
	}

	_ = server.Stop()
	return nil
}

func getTestCIDs(count int) []cid.Cid {
	const length = 32
	const code = mh.SHA2_256
	const base = "dhttest"
	const codecType = cid.Raw // TODO: is this right?

	cids := make([]cid.Cid, count)
	for i := 0; i < count; i++ {
		mh, err := mh.Sum(append([]byte(base), byte(i)), code, length)
		if err != nil {
			panic(err)
		}

		cids[i] = cid.NewCidV1(codecType, mh)
		log.Debugf("test CID: %s", cids[i])
	}
	return cids
}
