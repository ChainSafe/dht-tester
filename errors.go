package main

import (
	"errors"
)

var (
	errFailedToBootstrap = errors.New("failed to bootstrap to any bootnode")
)
