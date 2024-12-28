package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/nsqlite/nsqlite/internal/log"
	"github.com/nsqlite/nsqlite/internal/nsqlited/db"
	"github.com/nsqlite/nsqlite/internal/util/httputil"
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
	// AuthTokenAlgorithm is the algorithm to use for the auth token.
	AuthTokenAlgorithm string
	// AuthToken is the auth token to use.
	AuthToken string
}

// Server is the server for NSQLite.
type Server struct {
	Config
	isInitialized bool
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
	if config.AuthTokenAlgorithm == "" {
		config.AuthTokenAlgorithm = "plaintext"
	}

	s := Server{
		Config:        config,
		isInitialized: true,
		server:        http.Server{},
	}
	return &s, nil
}

// IsInitialized returns true if the server is initialized.
func (s *Server) IsInitialized() bool {
	return s.isInitialized
}

// createMux creates the HTTP mux for the server.
func (s *Server) createMux() *http.ServeMux {
	buildHandler := httputil.CreateHandlerFuncBuilder(s.errorHandler)
	mux := http.NewServeMux()

	headerAuthMws := []httputil.Middleware{
		s.queryHandlerAuthMiddleware,
	}

	routes := []struct {
		pattern     string
		handler     httputil.HandlerFuncErr
		middlewares []httputil.Middleware
	}{
		{
			pattern: "/health",
			handler: s.healthHandler,
		},
		{
			pattern:     "/version",
			handler:     s.versionHandler,
			middlewares: headerAuthMws,
		},
		{
			pattern:     "/stats",
			handler:     s.statsHandler,
			middlewares: headerAuthMws,
		},
		{
			pattern:     "/query",
			handler:     s.queryHandler,
			middlewares: headerAuthMws,
		},
	}

	setResponseHeaders := func(next httputil.HandlerFuncErr) httputil.HandlerFuncErr {
		return func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("x-server", "NSQLite")
			w.Header().Set("x-nsqlite", "true")
			return next(w, r)
		}
	}

	for _, route := range routes {
		route.middlewares = append(route.middlewares, setResponseHeaders)
		mux.HandleFunc(
			route.pattern, buildHandler(route.handler, route.middlewares...),
		)
	}

	return mux
}

// Start starts the server.
func (s *Server) Start() error {
	mux := s.createMux()
	addr := fmt.Sprintf("%s:%s", s.ListenHost, s.ListenPort)
	localAddr := fmt.Sprintf("http://%s:%s", "localhost", s.ListenPort)
	s.server = http.Server{
		Addr:    addr,
		Handler: mux,
	}

	s.Logger.InfoNs(log.NsServer, "server started at "+localAddr, log.KV{
		"listen_host": s.ListenHost,
		"listen_port": s.ListenPort,
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
