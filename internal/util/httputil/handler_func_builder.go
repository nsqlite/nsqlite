package httputil

import "net/http"

// HandlerFuncErr behaves like http.HandlerFunc but returns an error.
type HandlerFuncErr func(w http.ResponseWriter, r *http.Request) error

// Middleware wraps a HandlerFuncErr and returns a new one.
type Middleware func(next HandlerFuncErr) HandlerFuncErr

// ErrorHandler handles errors returned by handlers or middlewares.
type ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

// HandlerFuncBuilder
type HandlerFuncBuilder func(handler HandlerFuncErr, middlewares ...Middleware) http.HandlerFunc

// CreateHandlerFuncBuilder returns a function that creates an http.HandlerFunc
// by chaining middlewares and a final handler, using a centralized error handler.
func CreateHandlerFuncBuilder(errorHandler ErrorHandler) HandlerFuncBuilder {
	return func(handler HandlerFuncErr, middlewares ...Middleware) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			finalHandler := handler

			// Apply middlewares in reverse order for better readability
			// and clarity of the request flow.
			for i := len(middlewares) - 1; i >= 0; i-- {
				middleware := middlewares[i]
				previousHandler := finalHandler

				finalHandler = func(writer http.ResponseWriter, request *http.Request) error {
					return middleware(previousHandler)(writer, request)
				}
			}

			// Execute the final handler and handle the error if any
			if err := finalHandler(w, r); err != nil {
				errorHandler(w, r, err)
			}
		}
	}
}
