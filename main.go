package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-kad-dht"
	libp2phost "github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"

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
	flagPrefixLength  = "prefix-length"
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
				Value: 60,
			},
			&cli.BoolFlag{
				Name:  flagAutoTest,
				Usage: "automatically provide and look up test CIDs",
				Value: false,
			},
			&cli.UintFlag{
				Name:  flagPrefixLength,
				Usage: "set prefix length for lookups; set to 0 to look up full double-hash",
				Value: 0,
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
	prefixLength := int(c.Uint(flagPrefixLength))

	for i := 0; i < count; i++ {
		log.Infof("starting node %d", i)
		cfg := &config{
			Ctx:          context.Background(),
			Port:         uint16(basePort + i),
			Index:        i,
			AutoTest:     autoTest,
			PrefixLength: prefixLength,
		}

		h, err := NewHost(cfg)
		if err != nil {
			return err
		}

		bootnodes = append(bootnodes, h.addrInfo())
		hosts = append(hosts, h)
	}

	time.Sleep(time.Millisecond * 300)

	for i, h := range hosts {
		err := h.Start()
		if err != nil {
			return err
		}

		time.Sleep(time.Millisecond * 300)
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
		err := h.Stop()
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

type config struct {
	Ctx          context.Context
	Port         uint16
	KeyFile      string
	Index        int
	AutoTest     bool
	PrefixLength int
}

type host struct {
	ctx      context.Context
	cancel   context.CancelFunc
	h        libp2phost.Host
	dht      *dht.IpfsDHT
	autoTest bool
}

func NewHost(cfg *config) (*host, error) {
	if cfg.KeyFile == "" {
		cfg.KeyFile = path.Join(os.TempDir(), fmt.Sprintf("node-%d.key", cfg.Index))
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
		dht.PrefixLookups(cfg.PrefixLength),
		dht.Mode(dht.ModeAutoServer),
		dht.BootstrapPeersFunc(bootstrapPeersFunc),
	}...)
	if err != nil {
		return nil, err
	}

	ourCtx, cancel := context.WithCancel(cfg.Ctx)
	return &host{
		ctx:      ourCtx,
		cancel:   cancel,
		h:        h,
		dht:      dht,
		autoTest: cfg.AutoTest,
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

	ticker := time.NewTicker(time.Second * time.Duration(3+randDuration.Int64()))
	go func() {
		for {
			select {
			case <-h.ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if !h.autoTest {
					continue
				}

				h.provide([]cid.Cid{
					getRandTestCID(),
				})

				_ = h.lookup(getRandTestCID())
			}
		}
	}()

	return nil
}

func getRandTestCID() cid.Cid {
	randIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(cids))))
	if err != nil {
		panic(err)
	}

	return cids[randIdx.Int64()]
}

func (h *host) Stop() error {
	h.cancel()
	if err := h.h.Close(); err != nil {
		return fmt.Errorf("failed to close libp2p host: %w", err)
	}
	return nil
}

func (h *host) provide(cids []cid.Cid) {
	for _, cid := range cids {
		err := h.dht.Provide(h.ctx, cid, true)
		if err != nil {
			log.Warnf("%s failed to provide cid: %s", h.h.ID(), err)
			continue
		}

		log.Infof("%s provided cid %s", h.h.ID(), cid)
	}
}

func (h *host) lookup(target cid.Cid) []peer.AddrInfo {
	providers, err := h.dht.FindProviders(h.ctx, target)
	if err != nil {
		log.Warnf("failed to find any providers for cid %s: %s", target, err)
		return nil
	}

	// TODO: track providers and check for success/failure
	log.Infof("found providers for cid %s: %s", target, providers)
	return providers
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
