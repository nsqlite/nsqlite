package sysutil

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// ClearTerminal clears the terminal screen in supported operating systems.
func ClearTerminal() {
	goos := runtime.GOOS

	if strings.HasPrefix(goos, "windows") {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		_ = cmd.Run()
		return
	}

	if strings.HasPrefix(goos, "linux") || strings.HasPrefix(goos, "darwin") {
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		_ = cmd.Run()
	}
}
