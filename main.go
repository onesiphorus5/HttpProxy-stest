package main

import (
	"os"

	"github.com/onesiphorus5/HttpProxy-stest/proxy"
	"github.com/onesiphorus5/HttpProxy-stest/server"
)

func main() {
	mode := os.Getenv("MODE")
	switch mode {
	case "server":
		srv := server.New("8000")
		srv.Start()
	case "proxy":
		target := os.Getenv("TARGET")
		if target == "" {
			target = "localhost:8000" // Default for manual runs
		}
		pxy := proxy.New("8888", target)
		pxy.Start()
	default:
		panic("MODE must be 'server' or 'proxy'")
	}
	select {}
}
