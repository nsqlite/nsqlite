package repl

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nsqlite/nsqlite/internal/nsqlite/config"
	"github.com/nsqlite/nsqlite/internal/util/sysutil"
	"github.com/nsqlite/nsqlite/internal/version"
	"github.com/nsqlite/nsqlitego/nsqlitehttp"
	"github.com/peterh/liner"
)

type Repl struct {
	conf        config.Config
	client      *nsqlitehttp.Client
	ctx         context.Context
	stop        context.CancelFunc
	reader      *bufio.Reader
	txId        string
	historyPath string
}

func NewRepl(
	ctx context.Context,
	stop context.CancelFunc,
	conf config.Config,
	client *nsqlitehttp.Client,
) Repl {
	return Repl{
		conf:        conf,
		client:      client,
		ctx:         ctx,
		stop:        stop,
		reader:      bufio.NewReader(os.Stdin),
		historyPath: filepath.Join(os.TempDir(), ".nsqlite_history"),
	}
}

func (r *Repl) Start() error {
	remoteURL := r.conf.ParsedConnStr.String()

	if err := r.client.IsHealthy(); err != nil {
		return fmt.Errorf("failed to connect to %s: %w", remoteURL, err)
	}

	remoteVersion, err := r.client.Version()
	if err != nil {
		return fmt.Errorf("failed to get remote NSQLite version: %w", err)
	}

	fmt.Println()
	fmt.Printf("Connected to %s running NSQLite %s\n", remoteURL, remoteVersion)
	fmt.Println(`Enter ".help" for usage hints and ".quit" or "CTRL+C" to quit`)
	fmt.Println()

	if version.Version != remoteVersion {
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

			if input == "exit" || input == ".exit" || input == ".quit" {
				r.Shutdown()
				return nil
			}

			if input == "clear" || input == ".clear" {
				sysutil.ClearTerminal()
				continue
			}

			if input == "help" || input == ".help" {
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

// setTxId sets the current transaction ID for the REPL. Send empty string to
// reset the transaction ID.
func (r *Repl) setTxId(txId string) {
	r.txId = txId
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
	if r.txId != "" {
		txId := r.txId
		if len(txId) > 7 {
			txId = txId[len(txId)-7:]
		}
		label = fmt.Sprintf("NSQLite(%s)> ", txId)
	}

	line := liner.NewLiner()
	defer line.Close()
	line.SetCtrlCAborts(true)
	line.SetCompleter(cmdHelpCompleter)

	if file, err := os.Open(r.historyPath); err == nil {
		_, _ = line.ReadHistory(file)
		file.Close()
	} else {
		fmt.Println("No previous history.", err)
	}

	prompt, err := line.Prompt(label)
	if err != nil {
		if err == liner.ErrPromptAborted {
			fmt.Println("CTRL+C pressed, exiting...")
			return ".quit"
		}
		return ""
	}

	line.AppendHistory(prompt)
	if file, err := os.Create(r.historyPath); err == nil {
		_, _ = line.WriteHistory(file)
		file.Close()
	}

	return strings.TrimSpace(prompt)
}
