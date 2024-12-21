package config

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/alexflint/go-arg"
	"github.com/nsqlite/nsqlite/internal/version"
)

// Config represents the configuration for nsqlited.
type Config struct {
	HttpListenAddr   string `arg:"--http-listen-addr,env:NSQLITE_HTTP_LISTEN_ADDR" help:"HTTP listen address" default:"0.0.0.0"`
	HttpListenPort   string `arg:"--http-listen-port,env:NSQLITE_HTTP_LISTEN_PORT" help:"HTTP listen port" default:"9876"`
	AuthEnabled      bool   `arg:"--auth-enabled,env:NSQLITE_AUTH_ENABLED" help:"Enable auth" default:"false"`
	AuthAlgorithm    string `arg:"--auth-algorithm,env:NSQLITE_AUTH_ALGORITHM" help:"Auth hash algorithm for token (plaintext, sha256, bcrypt)" default:"plaintext"`
	AuthToken        string `arg:"--auth-token,env:NSQLITE_AUTH_TOKEN" help:"Auth token, required if auth enabled (should be hashed if algorithm is not plaintext)"`
	ReadOnlyPoolSize int    `arg:"--read-only-pool-size,env:NSQLITE_READ_ONLY_POOL_SIZE" help:"Number of connections for the read-only pool (default: number of CPUs * 2)"`
}

func (Config) Version() string {
	return fmt.Sprintf("%s\n", version.ServerVersion())
}

// MustParse parses and validates the configuration from the command
// line arguments. It returns a Config struct or exits the program
// with an error.
func MustParse(args []string) Config {
	cfg := Config{}

	parser, err := arg.NewParser(
		arg.Config{},
		&cfg,
	)
	if err != nil {
		log.Fatal(err)
	}
	parser.MustParse(args[1:])

	if err := validateListenAddr(cfg.HttpListenAddr); err != nil {
		log.Fatal(err)
	}

	if err := validateListenPort(cfg.HttpListenPort); err != nil {
		log.Fatal(err)
	}

	if err := validateAuthAlgorithm(cfg.AuthAlgorithm); err != nil {
		log.Fatal(err)
	}

	if cfg.AuthEnabled && cfg.AuthToken == "" {
		log.Fatal(errors.New("auth token is required if auth is enabled"))
	}

	return cfg
}

// validateListenAddr validates if addr is a valid ip address.
func validateListenAddr(addr string) error {
	re := regexp.MustCompile(`^([0-9]{1,3}\.){3}[0-9]{1,3}($|/[0-9]{2})$`)
	if !re.MatchString(addr) {
		return errors.New("invalid listen address")
	}
	return nil
}

// validateListenPort validates if port is a valid port number.
func validateListenPort(port string) error {
	re := regexp.MustCompile(`^\d{1,5}$`)
	if !re.MatchString(port) {
		return errors.New("invalid listen port, valid values are 1-65535")
	}
	return nil
}

// validateAuthAlgorithm validates if algorithm is a valid auth algorithm.
func validateAuthAlgorithm(algorithm string) error {
	valid := []string{"plaintext", "sha256", "bcrypt"}

	for _, v := range valid {
		if algorithm == v {
			return nil
		}
	}

	return fmt.Errorf(
		"invalid auth algorithm, valid values are: %s",
		strings.Join(valid, ", "),
	)
}
