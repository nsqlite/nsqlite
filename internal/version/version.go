package version

import "fmt"

const version = "v0.0.1"

const asciiArt = `
    _   _______ ____    __    _ __     
   / | / / ___// __ \  / /   (_) /____ 
  /  |/ /\__ \/ / / / / /   / / __/ _ \
 / /|  /___/ / /_/ / / /___/ / /_/  __/
/_/ |_//____/\___\_\/_____/_/\__/\___/`

// AsciiArt returns the ASCII art of nsqlited.
func AsciiArt() string {
	// This just removes the first newline character
	return asciiArt[1:]
}

// NSQLitedVersion returns the version of nsqlited.
func NSQLitedVersion() string {
	return fmt.Sprintf("%s\nServer %s", AsciiArt(), version)
}

// NSQLiteVersion returns the version of nsqlite.
func NSQLiteVersion() string {
	return fmt.Sprintf("%s\nClient %s", AsciiArt(), version)
}
