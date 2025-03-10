package main

import (
	"os"

	"HttpProxy-stest/proxy"
	"HttpProxy-stest/server"
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
			target = "localhost:8000"
		}
		pxy := proxy.New("8888", target)
		pxy.Start()
	default:
		panic("MODE must be 'server' or 'proxy'")
	}
	select {}
}
