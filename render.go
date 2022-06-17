package oauth

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// ErrorResponseType ...
type ErrorResponseType string

// See: https://datatracker.ietf.org/doc/html/rfc6749#section-4.1.2.1
// See: https://datatracker.ietf.org/doc/html/rfc6749#section-4.2.2.1
// See: https://datatracker.ietf.org/doc/html/rfc6749#section-5.2
const (
	// AuthorizationCodeGrantInvalidRequest The request is missing a required parameter, includes an invalid parameter
	// value, includes a parameter more than once, or is otherwise malformed.
	AuthorizationCodeGrantInvalidRequest ErrorResponseType = "invalid_request"
	// AuthorizationCodeGrantUnauthorizedClient This client is not authorized to use the requested grant type.
	// For example, if you restrict which applications can use the Implicit grant, you would return this error for the
	// other apps.
	AuthorizationCodeGrantUnauthorizedClient ErrorResponseType = "unauthorized_client"
	// AuthorizationCodeGrantAccessDenied The resource owner or authorization server denied the request.
	AuthorizationCodeGrantAccessDenied ErrorResponseType = "access_denied"
	// AuthorizationCodeGrantUnsupportedResponseType The authorization server does not support obtaining an
	// authorization code using this method.
	AuthorizationCodeGrantUnsupportedResponseType ErrorResponseType = "unsupported_response_type"
	// AuthorizationCodeGrantInvalidScope The requested scope is invalid, unknown, or malformed.
	AuthorizationCodeGrantInvalidScope ErrorResponseType = "invalid_scope"
	// AuthorizationCodeGrantServerError The authorization server encountered an unexpected condition that prevented it
	// from fulfilling the request. (This error code is needed because a 500 Internal Server Error HTTP status code
	// cannot be returned to the client via an HTTP redirect.)
	AuthorizationCodeGrantServerError ErrorResponseType = "server_error"
	// AuthorizationCodeGrantTemporarilyUnavailable The authorization server is currently unable to handle the request
	// due to a temporary overloading or maintenance of the server. (This error code is needed because a 503 Service
	// Unavailable HTTP status code cannot be returned to the client via an HTTP redirect.)
	AuthorizationCodeGrantTemporarilyUnavailable ErrorResponseType = "temporarily_unavailable"

	// ImplicitGrantInvalidRequest The request is missing a required parameter, includes an invalid parameter value,
	// includes a parameter more than once, or is otherwise malformed.
	ImplicitGrantInvalidRequest ErrorResponseType = "invalid_request"
	// ImplicitGrantUnauthorizedClient The client is not authorized to request an access token using this method.
	ImplicitGrantUnauthorizedClient ErrorResponseType = "unauthorized_client"
	// ImplicitGrantAccessDenied The resource owner or authorization server denied the request.
	ImplicitGrantAccessDenied ErrorResponseType = "access_denied"
	// ImplicitGrantUnsupportedResponseType The authorization server does not support obtaining an
	// authorization code using this method.
	ImplicitGrantUnsupportedResponseType ErrorResponseType = "unsupported_response_type"
	// ImplicitGrantInvalidScope The requested scope is invalid, unknown, or malformed.
	ImplicitGrantInvalidScope ErrorResponseType = "invalid_scope"

	// TokenInvalidRequest The request is missing a required parameter, includes an
	// unsupported parameter value (other than grant type), repeats a parameter, includes multiple credentials,
	// utilizes more than one mechanism for authenticating the client, or is otherwise malformed.
	TokenInvalidRequest ErrorResponseType = "invalid_request"
	// TokenInvalidClient Client authentication failed (e.g., unknown client, no
	// client authentication included, or unsupported authentication method). The authorization server MAY
	// return an HTTP 401 (Unauthorized) status code to indicate which HTTP authentication schemes are supported.
	// If the client attempted to authenticate via the "Authorization" request header field, the authorization server
	// MUST respond with an HTTP 401 (Unauthorized) status code and include the "WWW-Authenticate" response header field
	// matching the authentication scheme used by the client.
	TokenInvalidClient ErrorResponseType = "invalid_client"
	// TokenInvalidGrant The provided authorization grant (e.g., authorization
	// code, resource owner credentials) or refresh token is invalid, expired, revoked, does not match the redirection
	// URI used in the authorization request, or was issued to another client.
	TokenInvalidGrant ErrorResponseType = "invalid_grant"
	// TokenUnauthorizedClient The authenticated client is not authorized to use this authorization grant type.
	TokenUnauthorizedClient ErrorResponseType = "unauthorized_client"
	// TokenUnsupportedGrantType The authorization grant type is not supported by the
	// authorization server.
	TokenUnsupportedGrantType ErrorResponseType = "unsupported_grant_type"
	// TokenInvalidScope The requested scope is invalid, unknown, malformed, or
	// exceeds the scope granted by the resource owner.
	TokenInvalidScope ErrorResponseType = "invalid_scope"
	// TokenServerError The authorization server encountered an unexpected
	// condition that prevented it from fulfilling the request. (This error code is needed because a 500 Internal Server
	// Error HTTP status code cannot be returned to the client via an HTTP redirect.)
	TokenServerError ErrorResponseType = "server_error"
	// TokenTemporarilyUnavailable The authorization server is currently unable to handle
	// the request due to a temporary overloading or maintenance of the server.  (This error code is needed because a 503
	// Service Unavailable HTTP status code cannot be returned to the client via an HTTP redirect.)
	TokenTemporarilyUnavailable ErrorResponseType = "temporarily_unavailable"
)

type ErrorResponse struct {
	Error       ErrorResponseType `json:"error"`
	Description string            `json:"error_description"`
	URI         string            `json:"error_uri,omitempty"`
	State       string            `json:"state,omitempty"`
}

func renderError(w http.ResponseWriter, error ErrorResponseType, description, uri string, statusCode int) {
	renderJSON(w, ErrorResponse{Error: error, Description: description, URI: uri}, false, statusCode)
}

// renderJSON marshals 'v' to JSON, automatically escaping HTML, setting the
// Content-Type as application/json, and sending the status code header.
func renderJSON(w http.ResponseWriter, v interface{}, noStore bool, statusCode int) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if noStore {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.WriteHeader(statusCode)
	_, _ = w.Write(buf.Bytes())
}
