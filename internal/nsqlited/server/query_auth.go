package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/nsqlite/nsqlite/internal/util/cryptoutil"
	"github.com/nsqlite/nsqlite/internal/util/httputil"
)

// queryHandlerAuthMiddleware is a middleware that checks the Authorization
// header of the incoming request and compares it to the server's AuthToken
// configuration. If the AuthToken is empty, the middleware does nothing.
func (s *Server) queryHandlerAuthMiddleware(
	next httputil.HandlerFuncErr,
) httputil.HandlerFuncErr {
	return func(w http.ResponseWriter, r *http.Request) error {
		if s.AuthToken == "" {
			return next(w, r)
		}

		unauthorized := func() error {
			return httputil.NewJSONError(
				http.StatusUnauthorized, errors.New("Unauthorized"), "Unauthorized",
			)
		}

		clientAuthToken := r.Header.Get("Authorization")
		clientAuthToken = strings.TrimPrefix(clientAuthToken, "Bearer ")
		clientAuthToken = strings.TrimPrefix(clientAuthToken, "bearer ")
		if clientAuthToken == "" {
			return unauthorized()
		}

		if s.AuthTokenAlgorithm == "plaintext" {
			if checkPlaintextAuth(clientAuthToken, s.AuthToken) {
				return next(w, r)
			}
		}

		if s.AuthTokenAlgorithm == "argon2" {
			if checkArgon2Auth(clientAuthToken, s.AuthToken) {
				return next(w, r)
			}
		}

		if s.AuthTokenAlgorithm == "bcrypt" {
			if checkBcryptAuth(clientAuthToken, s.AuthToken) {
				return next(w, r)
			}
		}

		return unauthorized()
	}
}

// checkPlaintextAuth checks if the client token matches the server token
// in plaintext.
func checkPlaintextAuth(clientToken string, serverToken string) bool {
	return clientToken == serverToken
}

// checkArgon2Auth checks if the client token matches the server token
// using the Argon2 algorithm.
func checkArgon2Auth(clientToken string, serverToken string) bool {
	return cryptoutil.Argon2CheckHash(clientToken, serverToken)
}

// checkBcryptAuth checks if the client token matches the server token
// using the Bcrypt algorithm.
func checkBcryptAuth(clientToken string, serverToken string) bool {
	return cryptoutil.BcryptCheckHash(clientToken, serverToken)
}
