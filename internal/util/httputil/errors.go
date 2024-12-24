package httputil

// JSONError represents an error that can be safely marshaled to JSON.
type JSONError struct {
	error
	HTTPStatus  int
	SafeMessage string
}

// NewJSONError creates a new JSONError.
//
// The err is intended to be a detailed error to be internally logged, while
// safeMessage is a message that can be safely shown to the client without
// revealing too much information.
//
// If no safeMessage is provided, the textual representation of the status
// code will be used.
func NewJSONError(status int, err error, safeMessage ...string) JSONError {
	pickedSafeMessage := ""
	if len(safeMessage) > 0 {
		pickedSafeMessage = safeMessage[0]
	}

	return JSONError{
		error:       err,
		HTTPStatus:  status,
		SafeMessage: pickedSafeMessage,
	}
}
