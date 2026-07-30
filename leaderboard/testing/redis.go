package testing

import (
	"net"
	"strconv"

	"github.com/alicebob/miniredis/v2"
)

// RedisServer is an in-process Redis-compatible server for tests.
type RedisServer struct {
	*miniredis.Miniredis
	Host string
	Port int
}

// StartRedis starts an isolated Redis-compatible test server.
func StartRedis() (*RedisServer, error) {
	server, err := miniredis.Run()
	if err != nil {
		return nil, err
	}
	host, portString, err := net.SplitHostPort(server.Addr())
	if err != nil {
		server.Close()
		return nil, err
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		server.Close()
		return nil, err
	}
	return &RedisServer{Miniredis: server, Host: host, Port: port}, nil
}
