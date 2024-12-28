package version

import "fmt"

const (
	Version = "v0.0.1"

	colorReset      = "\033[0m"
	colorRed        = "\033[31m"
	colorRedBold    = "\033[31;1m"
	colorGreen      = "\033[32m"
	colorGreenBold  = "\033[32;1m"
	colorYellow     = "\033[33m"
	colorYellowBold = "\033[33;1m"
	colorBlue       = "\033[34m"
	colorBlueBold   = "\033[34;1m"
	colorPurple     = "\033[35m"
	colorPurpleBold = "\033[35;1m"
	colorCyan       = "\033[36m"
	colorCyanBold   = "\033[36;1m"
	colorWhite      = "\033[37m"
	colorWhiteBold  = "\033[37;1m"
)

// asciiArtTpl returns the ASCII art of nsqlited.
func asciiArtTpl() string {
	asciiArt := `
    _   _______ ____    __    _ __     
   / | / / ___// __ \  / /   (_) /____ 
  /  |/ /\__ \/ / / / / /   / / __/ _ \
 / /|  /___/ / /_/ / / /___/ / /_/  __/
/_/ |_//____/\___\_\/_____/_/\__/\___/
%s ` + Version + `
For more information visit https://github.com/nsqlite/nsqlite and please leave a star`

	asciiArt = asciiArt[1:]                          // This just removes the first newline character
	asciiArt = colorCyanBold + asciiArt + colorReset // Add color to the ASCII art

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
