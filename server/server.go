package server

import (
	"fmt"
	"net/http"
)

// Server wraps an http.Server with a start method.
type Server struct {
	*http.Server
}

// New creates a new server instance.
func New(port string) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Hello from the server!")
	})

	return &Server{
		Server: &http.Server{
			Addr:    ":" + port,
			Handler: mux,
		},
	}
}

// Start runs the server in a goroutine.
func (s *Server) Start() {
	go func() {
		fmt.Printf("Server running on %s...\n", s.Addr)
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("Server error:", err)
		}
	}()
}
