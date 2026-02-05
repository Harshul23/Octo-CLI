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

	// 3. The result should be the next port (blockedPort + 1) since blockedPort is busy
	if got != blockedPort+1 {
		t.Errorf("FindAvailablePort(%d) = %d; want %d (because %d is busy)", blockedPort, got, blockedPort+1, blockedPort)
	}
}
