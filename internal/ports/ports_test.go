package ports

import (
	"net"
	"testing"
)

func TestFindAvailablePort(t *testing.T) {
	// 1. Manually block a port
	ln, err := net.Listen("tcp", "localhost:3000")
	if err != nil {
		t.Fatalf("Failed to block port 3000 for testing: %v", err)
	}
	defer ln.Close()

	// 2. Ask Octo to find a port starting at 3000
	got := FindAvailablePort(3000)
	want := 3001

	if got != want {
		t.Errorf("FindAvailablePort(3000) = %d; want %d (because 3000 is busy)", got, want)
	}
}
