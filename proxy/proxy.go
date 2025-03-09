package proxy

import (
	"fmt"
	"io"
	"net"
	"net/http"
)

// Proxy wraps an http.Server with a start method.
type Proxy struct {
	*http.Server
	target string // Where to forward requests
}

// New creates a new proxy instance.
func New(port, target string) *Proxy {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := net.Dial("tcp", target)
		if err != nil {
			http.Error(w, "Failed to connect to server", http.StatusBadGateway)
			return
		}
		defer conn.Close()

		clientConn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			http.Error(w, "Failed to hijack connection", http.StatusInternalServerError)
			return
		}
		defer clientConn.Close()

		err = r.Write(conn)
		if err != nil {
			fmt.Println("Failed to forward request:", err)
			return
		}

		_, err = io.Copy(clientConn, conn)
		if err != nil {
			fmt.Println("Failed to forward response:", err)
		}
	})

	return &Proxy{
		Server: &http.Server{
			Addr:    ":" + port,
			Handler: mux,
		},
		target: target,
	}
}

// Start runs the proxy in a goroutine.
func (p *Proxy) Start() {
	go func() {
		fmt.Printf("Proxy running on %s...\n", p.Addr)
		if err := p.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("Proxy error:", err)
		}
	}()
}
