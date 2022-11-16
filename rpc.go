package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Server represents the JSON-RPC server
type Server struct {
	listener   net.Listener
	httpServer *http.Server
	nodeCount  int
}

// NewServer ...
func NewServer(hosts []*host) (*Server, error) {
	rpcServer := rpc.NewServer()
	rpcServer.RegisterCodec(NewCodec(), "application/json")

	s := newDHTService(hosts)
	if err := rpcServer.RegisterService(s, "dht"); err != nil {
		return nil, err
	}

	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", "localhost:9000") // TODO: make port configurable
	if err != nil {
		return nil, err
	}

	r := mux.NewRouter()
	r.Handle("/", rpcServer)

	headersOk := handlers.AllowedHeaders([]string{"content-type", "username", "password"})
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "OPTIONS"})
	originsOk := handlers.AllowedOrigins([]string{"*"})

	server := &http.Server{
		Addr:              ln.Addr().String(),
		ReadHeaderTimeout: time.Second,
		Handler:           handlers.CORS(headersOk, methodsOk, originsOk)(r),
	}

	return &Server{
		listener:   ln,
		httpServer: server,
	}, nil
}

// Start starts the JSON-RPC server.
func (s *Server) Start() error {
	log.Infof("Starting RPC server on %s", s.HttpURL())
	go func() {
		err := s.httpServer.Serve(s.listener)
		if err != nil {
			log.Warnf("server error: %s", err)
		}
	}()
	return nil
}

// Stop stops the JSON-RPC server.
func (s *Server) Stop() error {
	return s.httpServer.Close()
}

// HttpURL returns the URL used for HTTP requests
func (s *Server) HttpURL() string { //nolint:revive
	return fmt.Sprintf("http://%s", s.httpServer.Addr)
}

type DHTService struct {
	hosts []*host
}

func newDHTService(hosts []*host) *DHTService {
	return &DHTService{
		hosts: hosts,
	}
}

type NumHostsResponse struct {
	NumHosts int `json:"numHosts"`
}

func (s *DHTService) NumHosts(_ *http.Request, _ *interface{}, resp *NumHostsResponse) error {
	resp.NumHosts = len(s.hosts)
	return nil
}

type ProvideRequest struct {
	HostIndex int       `json:"hostIndex"`
	CIDs      []cid.Cid `json:"cids"`
}

func (s *DHTService) Provide(_ *http.Request, req *ProvideRequest, _ *interface{}) error {
	if req.HostIndex >= len(s.hosts) {
		return errors.New("host index too high")
	}

	s.hosts[req.HostIndex].provide(req.CIDs)
	return nil
}

type LookupRequest struct {
	HostIndex    int     `json:"hostIndex"`
	Target       cid.Cid `json:"cid"`
	PrefixLength int     `json:"prefixLength"`
}

type LookupResponse struct {
	Providers []peer.AddrInfo `json:"providers"`
}

func (s *DHTService) Lookup(_ *http.Request, req *LookupRequest, resp *LookupResponse) error {
	if req.HostIndex >= len(s.hosts) {
		return errors.New("host index too high")
	}

	provs, err := s.hosts[req.HostIndex].lookup(req.Target, req.PrefixLength)
	if err != nil {
		return err
	}

	resp.Providers = provs
	return nil
}

type IDRequest struct {
	HostIndex int `json:"hostIndex"`
}

type IDResponse struct {
	PeerID peer.ID `json:"peerID"`
}

func (s *DHTService) Id(_ *http.Request, req *IDRequest, resp *IDResponse) error {
	if req.HostIndex >= len(s.hosts) {
		return errors.New("host index too high")
	}

	resp.PeerID = s.hosts[req.HostIndex].h.ID()
	return nil
}
