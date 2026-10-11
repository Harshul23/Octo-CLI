package ports

import (
	"net"
	"testing"
)

func TestFindAvailablePort(t *testing.T) {
	// 1. Find a port that's currently available to use for testing
	//    We use port 0 to let the system assign an available port
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to get a test port: %v", err)
	}
	defer ln.Close()

	// Get the assigned port
	blockedPort := ln.Addr().(*net.TCPAddr).Port

	// 2. Ask Octo to find a port starting at the blocked port
	got := FindAvailablePort(blockedPort)

	// 3. The result should be an available port greater than the blocked port
	if got <= blockedPort || !IsPortAvailable(got) {
		t.Errorf("FindAvailablePort(%d) = %d; want an available port > %d", blockedPort, got, blockedPort)
	}
}
