package main

import (
	"simpanan/internal"

	"github.com/neovim/go-client/nvim/plugin"
)

func main() {
	plugin.Main(func(p *plugin.Plugin) error {
		p.HandleFunction(&plugin.FunctionOptions{Name: "SimpananRunQuery"}, internal.HandleRunQuery)
		p.HandleFunction(&plugin.FunctionOptions{Name: "SimpananGetConnections"}, internal.HandleGetConnections)
		p.HandleFunction(&plugin.FunctionOptions{Name: "SimpananAddConnection"}, internal.HandleAddConnection)
		return nil
	})
}
