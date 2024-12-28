package version

import (
	"fmt"
	"strings"

	"github.com/logrusorgru/aurora/v4"
)

const Version = "v0.0.1"

// asciiArtTpl returns the ASCII art of nsqlited.
func asciiArtTpl() string {
	lines := []string{
		`    _   _______ ____    __    _ __`,
		`   / | / / ___// __ \  / /   (_) /____`,
		`  /  |/ /\__ \/ / / / / /   / / __/ _ \`,
		` / /|  /___/ / /_/ / / /___/ / /_/  __/`,
		`/_/ |_//____/\___\_\/_____/_/\__/\___/`,
		`%s ` + Version,
		`For more information visit https://github.com/nsqlite/nsqlite and please leave a star`,
	}

	asciiArt := strings.Join(lines, "\n")
	asciiArt = aurora.Cyan(asciiArt).Bold().String()
	return asciiArt
}

// ServerVersion returns the server version of nsqlited.
func ServerVersion() string {
	return fmt.Sprintf(asciiArtTpl(), "Server")
}

// ClientVersion returns the client version of nsqlite.
func ClientVersion() string {
	return fmt.Sprintf(asciiArtTpl(), "CLI")
}
