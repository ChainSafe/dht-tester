package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ChainSafe/dht-tester/client"

	"github.com/ipfs/go-cid"
	"github.com/urfave/cli/v2"
)

var (
	flagCIDs         = "cids"
	flagTarget       = "cid"
	flagEndpoint     = "endpoint"
	flagHostIndex    = "host-index"
	flagPrefixLength = "prefix-length"

	app = &cli.App{
		Name:                 "dht-tester-cli",
		Usage:                "CLI for dht-tester",
		EnableBashCompletion: true,
		Suggest:              true,
		Commands: []*cli.Command{
			{
				Name:    "provide",
				Aliases: []string{"p"},
				Usage:   "provide CIDs",
				Action:  runProvide,
				Flags: []cli.Flag{
					cliFlagCIDs,
					cliFlagEndpoint,
					cliFlagHostIndex,
				},
			},
			{
				Name:    "lookup",
				Aliases: []string{"l"},
				Usage:   "look up providers for a CID",
				Action:  runLookup,
				Flags: []cli.Flag{
					cliFlagTarget,
					cliFlagEndpoint,
					cliFlagHostIndex,
					cliFlagPrefixLength,
				},
			},
			{
				Name:   "id",
				Usage:  "get peer ID for a specific host index",
				Action: runID,
				Flags: []cli.Flag{
					cliFlagEndpoint,
					cliFlagHostIndex,
				},
			},
		},
	}

	cliFlagCIDs = &cli.StringFlag{
		Name:  flagCIDs,
		Usage: "comma-separated list of CIDs to provide",
		Value: "",
	}

	cliFlagEndpoint = &cli.StringFlag{
		Name:  flagEndpoint,
		Usage: "endpoint of server",
		Value: "http://127.0.0.1:9000",
	}

	cliFlagTarget = &cli.StringFlag{
		Name:  flagTarget,
		Usage: "CID to look up",
		Value: "",
	}

	cliFlagHostIndex = &cli.IntFlag{
		Name:  flagHostIndex,
		Usage: "index of host which should provide/look up",
		Value: 0,
	}

	cliFlagPrefixLength = &cli.UintFlag{
		Name:  flagPrefixLength,
		Usage: "set prefix length for lookups; set to 0 to look up full double-hash",
		Value: 0,
	}

	errInvalidPrefixLength = errors.New("prefix-length must be less than 256")
)

func main() {
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func runProvide(c *cli.Context) error {
	cli := client.NewClient(c.String(flagEndpoint))

	cidsStr := c.String(flagCIDs)
	if cidsStr == "" {
		return errors.New("must provide --cids")
	}

	cidStrings := strings.Split(cidsStr, ",")
	cids := []cid.Cid{}
	for _, cidStr := range cidStrings {
		cid, err := cid.Decode(cidStr)
		if err != nil {
			fmt.Println("failed to decode CID string:", cidStr)
			continue
		}
		cids = append(cids, cid)
	}

	err := cli.Provide(c.Int(flagHostIndex), cids)
	if err != nil {
		return fmt.Errorf("failed to provide: %w", err)
	}

	return nil
}

func runLookup(c *cli.Context) error {
	cli := client.NewClient(c.String(flagEndpoint))

	cidStr := c.String(flagTarget)
	if cidStr == "" {
		return errors.New("must provide --cid")
	}

	target, err := cid.Decode(cidStr)
	if err != nil {
		return err
	}

	prefixLength := int(c.Uint(flagPrefixLength))
	if prefixLength > 256 {
		return errInvalidPrefixLength
	}

	providers, err := cli.Lookup(c.Int(flagHostIndex), target, prefixLength)
	if err != nil {
		return fmt.Errorf("failed to look up: %w", err)
	}

	fmt.Printf("found %d providers for cid %s\n", len(providers), target)
	for i, prov := range providers {
		fmt.Printf("\tprovider %d: %s\n", i, prov)
	}

	return nil
}

func runID(c *cli.Context) error {
	cli := client.NewClient(c.String(flagEndpoint))

	hostIndex := c.Int(flagHostIndex)
	id, err := cli.ID(hostIndex)
	if err != nil {
		return fmt.Errorf("failed to get peer ID: %w", err)
	}

	fmt.Printf("peer ID of host %d: %s\n", hostIndex, id)
	return nil
}
