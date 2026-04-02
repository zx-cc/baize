package main

import (
	"encoding/json"

	"github.com/zx-cc/baize/internal/collector/smart"
)

type collector interface {
	Collect() error
}

func main() {
	m, err := smart.GetSmartctlData(smart.Option{
		Type:   "megaraid",
		CtrlID: "0",
		Did:    "2",
	})

	if err != nil {
		panic(err)
	}
	res, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		panic(err)
	}

	println(string(res))
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
