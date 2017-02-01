package port

import (
	"errors"
	"fmt"
	"net"
)

const (
	minPort           = 0
	maxPort           = 65535
	minRegisteredPort = 1024
)

var previousPort = minRegisteredPort

// IsPortAvailable returns a flag is TCP port available.
func IsPortAvailable(port int) bool {
	if port < minPort || port > maxPort {
		return false
	}
	conn, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// GetPort gets a free TCP port between 1024-65535.
func GetPort() (int, error) {
	for i := previousPort; i < maxPort; i++ {
		if IsPortAvailable(i) {
			// Next previousPort is 2024 if i == 1024 now.
			previousPort = i + 1000
			return i, nil
		}
	}
	return -1, errors.New("Not found free TCP Port")
}
