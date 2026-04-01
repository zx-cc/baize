package main

import (
	"encoding/json"

	"github.com/zx-cc/baize/internal/collector/cpu"
)

func main() {
	c := cpu.New()

	if err := c.Collect(); err != nil {
		panic(err)
	}

	res, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		panic(err)
	}

	println(string(res))
}
