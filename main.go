package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/libp2p/go-libp2p"
	libp2phost "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-kad-dht"
	ma "github.com/multiformats/go-multiaddr"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"
	mh "github.com/multiformats/go-multihash"
	"github.com/urfave/cli/v2"
)

const cidCount = 64 // arbitrary

var log = logging.Logger("main")

var (
	flagCount    = "count"
	flagDuration = "duration"

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
				Value: 60,
			},
		},
	}
)

var cids []cid.Cid
var bootnodes []peer.AddrInfo

func bootstrapPeersFunc() []peer.AddrInfo {
	return bootnodes
}

func main() {
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	_ = logging.SetLogLevel("main", "debug")

	cids = getTestCIDs(cidCount)

	const basePort = 6000

	//bootnodes := []peer.AddrInfo{}
	hosts := []*host{}

	count := int(c.Uint(flagCount))

	for i := 0; i < count; i++ {
		log.Infof("starting node %d", i)
		cfg := &config{
			Ctx:   context.Background(),
			Port:  uint16(basePort + i),
			Index: i,
		}

		h, err := NewHost(cfg)
		if err != nil {
			return err
		}

		bootnodes = append(bootnodes, h.addrInfo())
		hosts = append(hosts, h)
	}

	time.Sleep(time.Second * 2)

	for i, h := range hosts {
		err := h.Start()
		if err != nil {
			return err
		}

		time.Sleep(time.Second)
		log.Infof("node %d started: %s", i, h.addrInfo())
	}

	duration, err := time.ParseDuration(fmt.Sprintf("%ds", c.Uint(flagDuration)))
	if err != nil {
		return err
	}
	<-time.After(duration)

	for _, h := range hosts {
		err := h.Stop()
		if err != nil {
			return err
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
	for i := 0; i < count; i++ {
		mh, err := mh.Sum(append([]byte(base), byte(i)), code, length)
		if err != nil {
			panic(err)
		}

		cids[i] = cid.NewCidV1(codecType, mh)
	}
	return cids
}

type config struct {
	Ctx     context.Context
	Port    uint16
	KeyFile string
	Index   int
}

type host struct {
	ctx    context.Context
	cancel context.CancelFunc
	h      libp2phost.Host
	dht    *dht.IpfsDHT
}

func NewHost(cfg *config) (*host, error) {
	if cfg.KeyFile == "" {
		cfg.KeyFile = fmt.Sprintf("node-%d.key", cfg.Index)
	}

	key, err := loadKey(cfg.KeyFile)
	if err != nil {
		log.Infof("failed to load libp2p key, generating key %s...", cfg.KeyFile)
		key, err = generateKey(0, cfg.KeyFile)
		if err != nil {
			return nil, err
		}
	}

	addr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", cfg.Port))
	if err != nil {
		return nil, err
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrs(addr),
		libp2p.Identity(key),
		libp2p.NATPortMap(),
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, err
	}

	dht, err := dht.New(cfg.Ctx, h, []dht.Option{
		//dht.PrefixLookups(0),
		dht.Mode(dht.ModeAutoServer),
		dht.BootstrapPeersFunc(bootstrapPeersFunc),
	}...)
	if err != nil {
		return nil, err
	}

	ourCtx, cancel := context.WithCancel(cfg.Ctx)
	return &host{
		ctx:    ourCtx,
		cancel: cancel,
		h:      h,
		dht:    dht,
	}, nil
}

func (h *host) addrInfo() peer.AddrInfo {
	return peer.AddrInfo{
		ID:    h.h.ID(),
		Addrs: h.h.Addrs(),
	}
}

func (h *host) Start() error {
	err := h.bootstrap()
	if err != nil {
		return err
	}

	randDuration, err := rand.Int(rand.Reader, big.NewInt(20))
	if err != nil {
		return err
	}

	randIdx, err := rand.Int(rand.Reader, big.NewInt(cidCount))
	if err != nil {
		return err
	}

	ticker := time.NewTicker(time.Second * time.Duration(10+randDuration.Int64()))
	go func() {
		for {
			select {
			case <-h.ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				cid := cids[randIdx.Int64()]
				err := h.dht.Provide(h.ctx, cid, true)
				if err != nil {
					log.Warnf("%s failed to provide cid: %s", h.h.ID(), err)
					continue
				}

				log.Infof("%s provided cid %s", h.h.ID(), cid)
			}
		}
	}()

	return nil
}

func (h *host) Stop() error {
	h.cancel()
	if err := h.h.Close(); err != nil {
		return fmt.Errorf("failed to close libp2p host: %w", err)
	}
	return nil
}

// bootstrap connects the host to the configured bootnodes
func (h *host) bootstrap() error {
	failed := 0
	for _, addrInfo := range bootnodes {
		if addrInfo.ID == h.h.ID() {
			continue
		}

		log.Debugf("bootstrapping to peer: peer=%s", addrInfo.ID)
		err := h.h.Connect(h.ctx, addrInfo)
		if err != nil {
			log.Debugf("failed to bootstrap to peer: err=%s", err)
			failed++
		}
	}

	if failed == len(bootnodes) && len(bootnodes) != 0 {
		return errFailedToBootstrap
	}

	time.Sleep(time.Second)
	log.Infof("%s peer count: %d", h.h.ID(), len(h.h.Network().Peers()))

	err := h.dht.Bootstrap(h.ctx)
	if err != nil {
		return err
	}

	return nil
}
