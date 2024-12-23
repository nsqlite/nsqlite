package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/nsqlite/nsqlite/internal/log"
	"github.com/nsqlite/nsqlite/internal/nsqlited/db"
)

// Config represents the configuration for a NSQLite server.
type Config struct {
	// Logger is the shared NSQLite logger.
	Logger log.Logger
	// Db is the NSQLite database instance to use.
	Db *db.DB
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
	server        http.Server
}

// NewServer creates a new NSQLite server.
func NewServer(config Config) (*Server, error) {
	if config.ListenHost == "" {
		config.ListenHost = "0.0.0.0"
	}
	if config.ListenPort == "" {
		config.ListenPort = "9876"
	}

	s := Server{
		isInitialized: true,
		logger:        config.Logger,
		listenHost:    config.ListenHost,
		listenPort:    config.ListenPort,
		server:        http.Server{},
	}
	return &s, nil
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
	s.server = http.Server{
		Addr:    addr,
		Handler: mux,
	}

	s.logger.InfoNs(log.NsServer, "server started at "+localAddr, log.KV{
		"listen_host": s.listenHost,
		"listen_port": s.listenPort,
	})

	err := s.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// Stop gracefully stops the server.
func (s *Server) Stop() error {
	return s.server.Shutdown(context.TODO())
}
