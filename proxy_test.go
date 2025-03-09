package main

import (
	"io"
	"net/http"
	"testing"
	"time"

	"HttpProxy-stest/proxy"
	"HttpProxy-stest/server"
)

func waitForServer(t *testing.T, url string) {
	t.Helper()
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatalf("Server at %s not ready after 5s", url)
		case <-ticker.C:
			resp, err := http.Get(url)
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return
			}
		}
	}
}

func TestProxyEndToEnd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		serverPort string
		proxyPort  string
		target     string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "BasicForwarding",
			serverPort: "8000",
			proxyPort:  "8888",
			target:     "localhost:8000",
			wantStatus: http.StatusOK,
			wantBody:   "Hello from the server!",
		},
		// Add more cases (e.g., server down, invalid target)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := server.New(tt.serverPort)
			srv.Start()
			defer srv.Close()
			waitForServer(t, "http://localhost:"+tt.serverPort+"/")

			pxy := proxy.New(tt.proxyPort, tt.target)
			pxy.Start()
			defer pxy.Close()
			waitForServer(t, "http://localhost:"+tt.proxyPort+"/")

			client := &http.Client{Timeout: 2 * time.Second}
			resp, err := client.Get("http://localhost:" + tt.proxyPort + "/")
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, resp.StatusCode)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response: %v", err)
			}
			if string(body) != tt.wantBody {
				t.Errorf("Expected response %q, got %q", tt.wantBody, string(body))
			}
		})
	}
}
