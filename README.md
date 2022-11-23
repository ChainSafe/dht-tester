# dht-tester

This is a small program used for running and testing the `Provide`/`FindProviders` functionality of go-libp2p-kad-dht.

## Requirements

- go 1.19

## Usage

To build (do these in `${GOPATH}/github.com/`):
```bash
git clone https://github.com/ChainSafe/dht-tester.git
git clone https://github.com/ChainSafe/go-libp2p-kad-dht.git
git clone https://github.com/ChainSafe/go-datastore.git
cd go-libp2p-kad-dht && git checkout noot/demo-logs && cd ..
git clone https://github.com/ChainSafe/go-libp2p-kbucket.git
cd dht-tester
go build
```

This places the `tester`, `client`, and `testclient` binaries in `bin/`.

### Tester

By default, `tester` runs an RPC server that exposes two RPC endpoints, `dht_provide` and `dht_lookup`. You can call these functions with the `cli` program to provide and look up CIDs.

`tester` also has an `--auto` mode where it will automatically provide and look up test CIDs. Note: in `--auto` mode, the RPC server still runs and accepts requests.

To run the tester with `<count>` nodes:
```bash
./bin/tester --count <count>
```

To run the tester in `--auto` mode:
```bash
./bin/tester --count <count> --auto
```

The tester has other options for `duration` and `prefix-length` (for double-hashing DHT prefix lookups):
```bash
$ ./bin/tester --help
NAME:
   dht-tester - test libp2p nodes running go-libp2p-kad-dht

USAGE:
   dht-tester [global options] command [command options] [arguments...]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --count value          number of nodes to run (default: 10)
   --duration value       length of time to run simulation in seconds (default: 60)
   --auto                 automatically provide and look up test CIDs (default: false)
   --prefix-length value  set prefix length for lookups; set to 0 to look up full double-hash (default: 0)
   --num-test-cids value  number of test CIDs to generate (default: 20)
   --log value            log level: one of [error|warn|info|debug] (default: "info")
   --help, -h             show help (default: false)
```

Tip: to print out generated test CIDs, turn on `--log=debug`.

### CLI

Once the tester is running, you can provide CIDs as follows:
```bash
./bin/client provide --cids bafkreihmx6mmapzpf3hqa63nsyu3kdyzymacw4ergtpro6xi5zetcc4k34,bafkreibxoxofljarx4aim62ku6rs4izji5g7r62yzfwcyptbr4hb36hnrm --host-index=<host-index>
```

You should see logs in `tester` saying the CID was provided.

The `provide` subcommand `--cids` flag takes a comma-separated list of CIDs to provide. The `--host-index` is the index of the node running in `tester` that should provide these CIDs (default=0). The `--host-index` must be less than `<count>`.

To look up providers for a CID:
```bash
./bin/client lookup --cid bafkreihmx6mmapzpf3hqa63nsyu3kdyzymacw4ergtpro6xi5zetcc4k34 --host-index=<host-index>
# found 2 providers for cid bafkreihmx6mmapzpf3hqa63nsyu3kdyzymacw4ergtpro6xi5zetcc4k34
#	provider 0: {12D3KooWKwiBxSXpjPEy8XNsP12fG5p2rj4sVBiJU6KMXt1XgrRV: [/ip4/192.168.0.102/tcp/6002 /ip4/127.0.0.1/tcp/6002]}
#	provider 1: {12D3KooWCxi2eugv2XHNeoeFyenfZ6F9UXLgZZZUFxy9iMBwgNVi: [/ip4/192.168.0.102/tcp/6000 /ip4/127.0.0.1/tcp/6000]}
```

### testclient

`Testclient` is an extension to the CLI that automatically provides a specified number of CIDs in round-robin fashion (ie. if there are 100 nodes and 1000 CIDs, each node will provide 10 CIDs). It also then does a lookup on every node and ensures that each node can find the correct providers for the CID.

First, run the tester with some number of nodes:
```bash
./bin/tester --count=50 --duration=6000 --num-test-cids=0 
```

Then, run `testclient`:
```bash
./testclient --num-test-cids=100
```

If all is successful, the programs exits quietly. Otherwise, it panics at the lookup that was missing providers.