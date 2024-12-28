package repl

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/nsqlite/nsqlite/internal/nsqlite/client"
	"github.com/nsqlite/nsqlite/internal/nsqlite/config"
	"github.com/nsqlite/nsqlite/internal/util/sysutil"
	"github.com/nsqlite/nsqlite/internal/version"
)

type Repl struct {
	conf       config.Config
	clientInst client.Client
	ctx        context.Context
	stop       context.CancelFunc
	reader     *bufio.Reader
}

func NewRepl(
	ctx context.Context,
	stop context.CancelFunc,
	conf config.Config,
	clientInst client.Client,
) Repl {
	return Repl{
		conf:       conf,
		clientInst: clientInst,
		ctx:        ctx,
		stop:       stop,
		reader:     bufio.NewReader(os.Stdin),
	}
}

func (r *Repl) Start() error {
	remoteURL := r.conf.ParsedConnectionString.String()

	if err := r.clientInst.IsHealthy(); err != nil {
		return fmt.Errorf("failed to connect to %s: %w", remoteURL, err)
	}

	remoteVersion, isDifferentVersion, err := r.clientInst.RemoteVersion()
	if err != nil {
		return fmt.Errorf("failed to get remote NSQLite version: %w", err)
	}

	fmt.Println()
	fmt.Printf("Connected to %s running NSQLite %s\n", remoteURL, remoteVersion)
	fmt.Println(`Enter ".help" for usage hints and ".quit" or "CTRL+C" to quit`)
	fmt.Println()

	if isDifferentVersion {
		fmt.Printf(
			"Warning: Your client version is %s, but the server is running %s\n",
			version.Version, remoteVersion,
		)
		fmt.Println("To avoid compatibility issues, consider using the same version on both sides")
		fmt.Println()
	}

	for {
		select {
		case <-r.ctx.Done():
			return nil
		default:
			input := r.prompt()

			if input == "" {
				continue
			}

			if input == ".exit" || input == ".quit" {
				r.Shutdown()
				return nil
			}

			if input == ".clear" || input == "clear" {
				sysutil.ClearTerminal()
				continue
			}

			if input == ".help" {
				cmdHelp()
				continue
			}

			if input == ".tables" {
				cmdQuery(r, `SELECT name FROM sqlite_master WHERE type = "table"`)
				continue
			}

			if strings.HasPrefix(input, ".") {
				fmt.Println("Unknown command, type .help for usage hints")
				continue
			}

			cmdQuery(r, input)
		}
	}
}

// Shutdown stops the REPL.
func (r *Repl) Shutdown() {
	r.stop()
}

// cleanError removes the unwanted text from the error message. So, the error
// is more readable.
func (r *Repl) cleanError(errStr string) string {
	errStr = strings.ReplaceAll(errStr, "failed to detect query type:", "")
	errStr = strings.ReplaceAll(errStr, "failed to prepare statement:", "")
	return strings.TrimSpace(errStr)
}

// prompt shows the prompt and reads the input from the user.
func (r *Repl) prompt() string {
	label := "NSQLite> "
	fmt.Print(label)

	text, _ := r.reader.ReadString('\n')
	return strings.TrimSpace(text)
}
