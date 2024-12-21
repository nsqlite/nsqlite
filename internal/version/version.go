package version

import "fmt"

const version = "v0.0.1"
const asciiArt = `
    _   _______ ____    __    _ __     
   / | / / ___// __ \  / /   (_) /____ 
  /  |/ /\__ \/ / / / / /   / / __/ _ \
 / /|  /___/ / /_/ / / /___/ / /_/  __/
/_/ |_//____/\___\_\/_____/_/\__/\___/
%s ` + version + `

For more information visit https://github.com/nsqlite/nsqlite and if you like the project, please leave a star on GitHub.`

// AsciiArt returns the ASCII art of nsqlited.
func AsciiArt() string {
	// This just removes the first newline character
	return asciiArt[1:]
}

// ServerVersion returns the server version of nsqlited.
func ServerVersion() string {
	return fmt.Sprintf(AsciiArt(), "Server")
}

// ClientVersion returns the client version of nsqlite.
func ClientVersion() string {
	return fmt.Sprintf(AsciiArt(), "Client")
}
