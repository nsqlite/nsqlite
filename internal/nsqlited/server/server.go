package server

import (
	"fmt"
	"net/http"

	"github.com/nsqlite/nsqlite/internal/log"
)

// Config represents the configuration for a NSQLite server.
type Config struct {
	// Logger is the shared NSQLite logger.
	Logger log.Logger
	// ListenHost is the host to listen on.
	ListenHost string
	// ListenPort is the port to listen on.
	ListenPort string
}

// Server is the server for NSQLite.
type Server struct {
	isInitialized bool
	logger        log.Logger
	listenHost    string
	listenPort    string
}

// NewServer creates a new NSQLite server.
func NewServer(config Config) (*Server, error) {
	if config.ListenHost == "" {
		config.ListenHost = "0.0.0.0"
	}
	if config.ListenPort == "" {
		config.ListenPort = "9876"
	}

	serv := Server{
		isInitialized: true,
		logger:        config.Logger,
		listenHost:    config.ListenHost,
		listenPort:    config.ListenPort,
	}
	return &serv, nil
}

// IsInitialized returns true if the server is initialized.
func (s *Server) IsInitialized() bool {
	return s.isInitialized
}

// Start starts the server.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	addr := fmt.Sprintf("%s:%s", s.listenHost, s.listenPort)
	localAddr := fmt.Sprintf("http://%s:%s", "localhost", s.listenPort)

	s.logger.InfoNs(log.NsServer, "server started at "+localAddr, log.KV{
		"address": addr,
	})
	return http.ListenAndServe(addr, mux)
}
