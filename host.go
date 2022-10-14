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
)

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

func newHost(cfg *config) (*host, error) {
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

func (h *host) start() error {
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

func (h *host) stop() error {
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
