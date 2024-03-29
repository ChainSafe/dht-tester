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
	_ = logging.SetLogLevel("main", "info")

	cids = getTestCIDs(c.Int(flagTestCIDsCount))

	client := client.NewClient(c.String(flagEndpoint))

	numHosts, err := client.NumHosts()
	if err != nil {
		return err
	}

	provides := make(map[cid.Cid][]peer.ID)

	// get at least one host to provide each test CID
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

		providers, has := provides[c]
		if !has {
			provides[c] = []peer.ID{id}
		} else {
			provides[c] = append(providers, id)
		}

		idx = (i + numHosts/2) % numHosts
		err = client.Provide(idx, []cid.Cid{c})
		if err != nil {
			return err
		}

		id, err = client.ID(idx)
		if err != nil {
			return err
		}

		providers, has = provides[c]
		if !has {
			provides[c] = []peer.ID{id}
		} else {
			provides[c] = append(providers, id)
		}
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
	keyIdx := 0
	for key, provs := range provides {
		provsMap := make(map[peer.ID]struct{})
		for _, p := range provs {
			provsMap[p] = struct{}{}
		}

		for i := 0; i < numHosts; i++ {
			// TODO: vary prefix lengths also
			prefixLength := 33
			found, err := c.Lookup(i, key, prefixLength)
			if err != nil {
				return fmt.Errorf("%d: lookup for key %s at host %d failed: %s", keyIdx, key, i, err)
			}

			if len(found) == 0 {
				return fmt.Errorf("%d: failed to find providers for key %s at host %d", keyIdx, key, i)
			}

			// if len(found) != len(provs) {
			// 	return fmt.Errorf("%d: found providers length %d didn't match expected %d", keyIdx, len(found), len(provs))
			// }

			// check peer IDs
			for _, f := range found {
				_, has := provsMap[f.ID]
				if !has {
					return fmt.Errorf("%d: found provider that doesn't have key %s at host %d", keyIdx, key, i)
				}
			}

		}
		keyIdx++
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
		log.Infof("test CID: %s %08b", cids[i], (cids[i].Bytes())[:5])
	}
	return cids
}
