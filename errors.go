package main

import (
	"errors"
)

var (
	errFailedToBootstrap   = errors.New("failed to bootstrap to any bootnode")
	errInvalidPrefixLength = errors.New("prefix-length must be less than 32")
)
