package main

import (
	"encoding/json"

	"github.com/zx-cc/baize/internal/collector/memory"
	"github.com/zx-cc/baize/internal/collector/network"
)

type collector interface {
	Collect() error
}

func main() {
	n := network.New()
	m := memory.New()

	print(n)
	print(m)
}

func print(n collector) {
	if err := n.Collect(); err != nil {
		panic(err)
	}

	res, err := json.MarshalIndent(n, "", "  ")
	if err != nil {
		panic(err)
	}

	println(string(res))
}
