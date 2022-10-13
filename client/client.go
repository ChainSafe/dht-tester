package client

import (
	"encoding/json"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"

	rpc "github.com/noot/go-json-rpc"
)

// Client represents a swap RPC client, used to interact with a swap daemon via JSON-RPC calls.
type Client struct {
	endpoint string
}

// NewClient ...
func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
	}
}

type ProvideRequest struct {
	HostIndex int       `json:"hostIndex"`
	CIDs      []cid.Cid `json:"cids"`
}

func (c *Client) Provide(hostIndex int, cids []cid.Cid) error {
	const method = "dht_provide"

	req := &ProvideRequest{
		HostIndex: hostIndex,
		CIDs:      cids,
	}

	params, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := rpc.PostRPC(c.endpoint, method, string(params))
	if err != nil {
		return fmt.Errorf("failed to post: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("server error: %w", resp.Error)
	}

	return nil
}

type LookupRequest struct {
	HostIndex int     `json:"hostIndex"`
	Target    cid.Cid `json:"cid"`
}

type LookupResponse struct {
	Providers []peer.AddrInfo
}

func (c *Client) Lookup(hostIndex int, target cid.Cid) ([]peer.AddrInfo, error) {
	const method = "dht_lookup"

	req := &LookupRequest{
		HostIndex: hostIndex,
		Target:    target,
	}

	params, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := rpc.PostRPC(c.endpoint, method, string(params))
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, resp.Error
	}

	var res *LookupResponse
	if err = json.Unmarshal(resp.Result, &res); err != nil {
		return nil, err
	}

	return res.Providers, nil
}
