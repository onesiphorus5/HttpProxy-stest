package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

func setupTestEnv(t *testing.T) (serverAddr, proxyAddr string, cleanup func()) {
	t.Helper()

	// Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create Docker client: %v", err)
	}

	// Build and run server container
	exec.Command("docker", "build", "-f", "Dockerfile.server", "-t", "server:test", ".").Run()
	serverResp, err := cli.ContainerCreate(context.Background(), &container.Config{
		Image: "server:test",
		Env:   []string{"MODE=server"},
		ExposedPorts: nat.PortSet{
			"8000/tcp": struct{}{},
		},
	}, &container.HostConfig{
		PortBindings: nat.PortMap{
			"8000/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: ""}}, // Dynamic port
		},
	}, nil, nil, "")
	if err != nil {
		t.Fatalf("Failed to create server container: %v", err)
	}
	if err := cli.ContainerStart(context.Background(), serverResp.ID, container.StartOptions{}); err != nil {
		t.Fatalf("Failed to start server container: %v", err)
	}
	serverInspect, err := cli.ContainerInspect(context.Background(), serverResp.ID)
	if err != nil {
		t.Fatalf("Failed to inspect server container: %v", err)
	}
	serverPort := serverInspect.NetworkSettings.Ports["8000/tcp"][0].HostPort
	serverAddr = "localhost:" + serverPort
	waitForServer(t, "http://"+serverAddr+"/")

	// Build and run proxy container
	exec.Command("docker", "build", "-f", "Dockerfile.proxy", "-t", "proxy:test", ".").Run()
	proxyResp, err := cli.ContainerCreate(context.Background(), &container.Config{
		Image: "proxy:test",
		Env:   []string{"MODE=proxy", fmt.Sprintf("TARGET=server:%s", serverPort)},
		ExposedPorts: nat.PortSet{
			"8888/tcp": struct{}{},
		},
	}, &container.HostConfig{
		PortBindings: nat.PortMap{
			"8888/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: ""}}, // Dynamic port
		},
		Links: []string{serverResp.ID + ":server"}, // Link proxy to server
	}, nil, nil, "")
	if err != nil {
		t.Fatalf("Failed to create proxy container: %v", err)
	}
	if err := cli.ContainerStart(context.Background(), proxyResp.ID, container.StartOptions{}); err != nil {
		t.Fatalf("Failed to start proxy container: %v", err)
	}
	proxyInspect, err := cli.ContainerInspect(context.Background(), proxyResp.ID)
	if err != nil {
		t.Fatalf("Failed to inspect proxy container: %v", err)
	}
	proxyPort := proxyInspect.NetworkSettings.Ports["8888/tcp"][0].HostPort
	proxyAddr = "localhost:" + proxyPort
	waitForServer(t, "http://"+proxyAddr+"/")

	// Cleanup
	cleanup = func() {
		cli.ContainerStop(context.Background(), serverResp.ID, container.StopOptions{})
		cli.ContainerRemove(context.Background(), serverResp.ID, container.RemoveOptions{})
		cli.ContainerStop(context.Background(), proxyResp.ID, container.StopOptions{})
		cli.ContainerRemove(context.Background(), proxyResp.ID, container.RemoveOptions{})
	}
	return serverAddr, proxyAddr, cleanup
}

func waitForServer(t *testing.T, url string) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatalf("Server at %s not ready after 10s", url)
		case <-ticker.C:
			resp, err := client.Get(url)
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return
			}
		}
	}
}

func TestProxyEndToEnd(t *testing.T) {
	t.Parallel()

	// Setup
	// serverAddr, proxyAddr, cleanup := setupTestEnv(t)
	_, proxyAddr, cleanup := setupTestEnv(t)
	defer cleanup()

	// Exercise
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + proxyAddr + "/")
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Validate
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	expected := "Hello from the server!"
	if string(body) != expected {
		t.Errorf("Expected response %q, got %q", expected, string(body))
	}
}
