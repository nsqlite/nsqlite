package config

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/nsqlite/nsqlite/internal/version"
)

// Config represents the configuration for nsqlited.
type Config struct {
	DataDirectory        string        `arg:"--data-directory,env:NSQLITE_DATA_DIRECTORY" help:"Directory for NSQLite database files" default:"./data"`
	AuthTokenAlgorithm   string        `arg:"--auth-token-algorithm,env:NSQLITE_AUTH_TOKEN_ALGORITHM" help:"Hash algorithm for the auth token (plaintext, argon2, bcrypt)" default:"plaintext"`
	AuthToken            string        `arg:"--auth-token,env:NSQLITE_AUTH_TOKEN" help:"Pre-hashed auth token; leave empty to disable authentication"`
	DisableOptimizations bool          `arg:"--disable-optimizations,env:NSQLITE_DISABLE_OPTIMIZATIONS" help:"Disable performance optimizations at startup for the underlying SQLite database, allowing manual tuning" default:"false"`
	ListenAddr           string        `arg:"--listen-addr,env:NSQLITE_LISTEN_ADDR" help:"Address for the server to listen on" default:"0.0.0.0"`
	ListenPort           string        `arg:"--listen-port,env:NSQLITE_LISTEN_PORT" help:"Port for the server to listen on" default:"9876"`
	TransactionTimeout   time.Duration `arg:"--transaction-timeout,env:NSQLITE_TRANSACTION_TIMEOUT" help:"If a transaction is not active for this duration, it will be rolled back. Valid time units are ns, us (or µs), ms, s, m, h" default:"5m"`
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

	if err := validateListenAddr(cfg.ListenAddr); err != nil {
		log.Fatal(err)
	}

	if err := validateListenPort(cfg.ListenPort); err != nil {
		log.Fatal(err)
	}

	if err := validateAuthTokenAlgorithm(cfg.AuthTokenAlgorithm); err != nil {
		log.Fatal(err)
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

// validateAuthTokenAlgorithm validates if algorithm is a valid auth algorithm.
func validateAuthTokenAlgorithm(algorithm string) error {
	valid := []string{"plaintext", "argon2", "bcrypt"}

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

// validateTransactionTimeout validates if timeout is greater than zero.
func validateTransactionTimeout(timeout time.Duration) error {
	if timeout <= 0 {
		return errors.New("invalid transaction timeout, must be greater than zero")
	}
	return nil
}
