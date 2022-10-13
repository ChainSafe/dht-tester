package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"

	"github.com/ipfs/go-cid"
)

// Server represents the JSON-RPC server
type Server struct {
	listener   net.Listener
	httpServer *http.Server
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

	server := &http.Server{
		Addr:              ln.Addr().String(),
		ReadHeaderTimeout: time.Second,
	}

	return &Server{
		listener:   ln,
		httpServer: server,
	}, nil
}

// Start starts the JSON-RPC and Websocket server.
func (s *Server) Start() error {
	log.Infof("Starting RPC server on %s", s.HttpURL())
	err := s.httpServer.Serve(s.listener) // Serve never returns nil
	return fmt.Errorf("RPC server failed: %w", err)
}

// Stop stops the JSON-RPC and websockets server.
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

type ProvideRequest struct {
	CIDs []cid.Cid `json:"cids"`
}

func (s *DHTService) Provide(_ *http.Request, req *ProvideRequest, _ *interface{}) error {
	return nil
}
