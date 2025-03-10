package main

import (
	"context"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

func setupTestEnv(t *testing.T) (serverAddr, proxyAddr string, cleanup func()) {
	t.Helper()

	// Docker client for server
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
			"8000/tcp": []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: ""}},
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

	// Get Docker host IP (assumes default bridge network)
	serverIP := "172.17.0.1" // Adjust if needed

	// Setup KVM VM for proxy
	cmd := exec.Command("bash", "vm-setup.sh", serverIP)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to setup VM: %v\nOutput: %s", err, output)
	}
	var vmIP string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "VM IP:") {
			vmIP = strings.TrimPrefix(line, "VM IP: ")
			break
		}
	}
	if vmIP == "" {
		t.Fatal("Failed to extract VM IP")
	}
	proxyAddr = vmIP + ":8888"
	waitForServer(t, "http://"+proxyAddr+"/")

	// Cleanup
	cleanup = func() {
		cli.ContainerStop(context.Background(), serverResp.ID, container.StopOptions{})
		cli.ContainerRemove(context.Background(), serverResp.ID, container.RemoveOptions{})
		exec.Command("virsh", "destroy", "proxy-vm").Run()
		exec.Command("virsh", "undefine", "proxy-vm").Run()
	}
	return serverAddr, proxyAddr, cleanup
}

func waitForServer(t *testing.T, url string) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	timeout := time.After(30 * time.Second) // Longer for VM
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatalf("Server at %s not ready after 30s", url)
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

	_, proxyAddr, cleanup := setupTestEnv(t)
	defer cleanup()

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + proxyAddr + "/")
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

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
