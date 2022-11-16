package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/ChainSafe/dht-tester/client"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p/core/peer"
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
	flagEndpoint      = "endpoint"

	cliFlagEndpoint = &cli.StringFlag{
		Name:  flagEndpoint,
		Usage: "endpoint of server",
		Value: "http://127.0.0.1:9000",
	}

	app = &cli.App{
		Name:                 "dht-tester",
		Usage:                "test libp2p nodes running go-libp2p-kad-dht",
		Action:               run,
		EnableBashCompletion: true,
		Suggest:              true,
		Flags: []cli.Flag{
			&cli.UintFlag{
				Name:  flagDuration,
				Usage: "length of time to run simulation in seconds",
				Value: 600,
			},
			&cli.IntFlag{
				Name:  flagTestCIDsCount,
				Usage: "number of test CIDs to generate",
				Value: 20,
			},
			cliFlagEndpoint,
		},
	}
)

// test CIDs generated at startup
var cids []cid.Cid

func main() {
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	cids = getTestCIDs(c.Int(flagTestCIDsCount))

	client := client.NewClient(c.String(flagEndpoint))

	numHosts, err := client.NumHosts()
	if err != nil {
		return err
	}

	provides := make(map[cid.Cid][]peer.ID)

	// get 1 host to provide each test CID
	// TODO: update to have random subset of hosts provide the CID
	for i, c := range cids {
		idx := i % numHosts
		err = client.Provide(idx, []cid.Cid{c})
		if err != nil {
			return err
		}

		id, err := client.ID(idx)
		if err != nil {
			return err
		}
		provides[c] = []peer.ID{id}
	}

	doneCh := make(chan struct{})
	go func() {
		err := lookup(client, provides, numHosts, doneCh)
		if err != nil {
			panic(err)
		}
	}()

	duration, err := time.ParseDuration(fmt.Sprintf("%ds", c.Uint(flagDuration)))
	if err != nil {
		return err
	}

	select {
	case <-time.After(duration):
	case <-doneCh:
	}

	return nil
}

func lookup(c *client.Client, provides map[cid.Cid][]peer.ID, numHosts int, doneCh chan<- struct{}) error {
	defer close(doneCh)
	for key, provs := range provides {
		for i := 0; i < numHosts; i++ {
			// TODO: vary prefix lengths also
			found, err := c.Lookup(i, key, 0)
			if err != nil {
				return fmt.Errorf("lookup for key %s at host %d failed: %s", key, i, err)
			}

			if len(found) != len(provs) {
				return fmt.Errorf("found providers length %d didn't match expected %d", len(found), len(provs))
			}
			// TODO check peer IDs
		}
	}

	return nil
}

func getTestCIDs(count int) []cid.Cid {
	const length = 32
	const code = mh.SHA2_256
	const base = "dhttest"
	const codecType = cid.Raw // TODO: is this right?

	cids := make([]cid.Cid, count)
	var buf [8]byte
	for i := 0; i < count; i++ {
		binary.LittleEndian.PutUint64(buf[:], uint64(i))
		mh, err := mh.Sum(append([]byte(base), buf[:]...), code, length)
		if err != nil {
			panic(err)
		}

		cids[i] = cid.NewCidV1(codecType, mh)
		log.Debugf("test CID: %s", cids[i])
	}
	return cids
}
