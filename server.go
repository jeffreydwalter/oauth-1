package oauth

import (
	"net/http"
	"time"

	"github.com/gofrs/uuid"
)

type GrantType string

const (
	PasswordGrant          GrantType = "password"
	ClientCredentialsGrant GrantType = "client_credentials"
	AuthCodeGrant          GrantType = "authorization_code"
	RefreshTokenGrant      GrantType = "refresh_token"
)

// CredentialsVerifier defines the interface of the user and client credentials verifier.
type CredentialsVerifier interface {
	// ValidateUser validates username and password returning an error if the user credentials are wrong
	ValidateUser(username, password, scope string, r *http.Request) error
	// ValidateClient validates clientID and secret returning an error if the client credentials are wrong
	ValidateClient(clientID, clientSecret, scope string, r *http.Request) error
	// AddClaims provides additional claims to the token
	AddClaims(tokenType TokenType, credential, tokenID, scope string, r *http.Request) (Claims, error)
	// AddProperties provides additional information to the authorization server response
	AddProperties(tokenType TokenType, credential, tokenID, scope string, r *http.Request) (Properties, error)
	// ValidateTokenID optionally validates previously stored tokenID during refresh request
	ValidateTokenID(tokenType TokenType, credential, tokenID, refreshTokenID string) error
	// StoreTokenID optionally stores the tokenID generated for the user
	StoreTokenID(tokenType TokenType, credential, tokenID, refreshTokenID string) error
}

// AuthorizationCodeVerifier defines the interface of the Authorization Code verifier
type AuthorizationCodeVerifier interface {
	// ValidateCode checks the authorization code and returns the user credential
	ValidateCode(clientID, clientSecret, code, redirectURI string, r *http.Request) (string, error)
}

// BearerServer is the OAuth 2 bearer server implementation.
type BearerServer struct {
	secretKey       string
	TokenTTL        time.Duration
	RefreshTokenTTL time.Duration
	verifier        CredentialsVerifier
	provider        *TokenProvider
}

// NewBearerServer creates new OAuth 2 bearer server
func NewBearerServer(secretKey string, ttl, refreshTTL time.Duration, verifier CredentialsVerifier, formatter TokenSecureFormatter) *BearerServer {
	if formatter == nil {
		formatter = NewSHA256RC4TokenSecurityProvider([]byte(secretKey))
	}
	return &BearerServer{
		secretKey:       secretKey,
		TokenTTL:        ttl,
		RefreshTokenTTL: refreshTTL,
		verifier:        verifier,
		provider:        NewTokenProvider(formatter)}
}

// UserCredentials manages password grant type requests
func (bs *BearerServer) UserCredentials(w http.ResponseWriter, r *http.Request) {
	grantType := r.FormValue("grant_type")
	scope := r.FormValue("scope")
	// get username and password from basic authorization header
	username, password, err := GetBasicAuthentication(r)
	if err != nil {
		renderError(w, TokenInvalidClient, "invalid username or password", "", http.StatusUnauthorized)
		return
	}

	if username == "" || password == "" {
		// Including the client credentials in the request-body using the two
		// parameters is NOT RECOMMENDED and SHOULD be limited to clients unable
		// to directly utilize the HTTP Basic authentication scheme
		username = r.FormValue("username")
		password = r.FormValue("password")
	}

	refreshToken := r.FormValue("refresh_token")
	resp, statusCode := bs.generateTokenResponse(GrantType(grantType), username, password, refreshToken, scope, "", "", r)
	renderJSON(w, resp, GrantType(grantType) == RefreshTokenGrant, statusCode)
}

// ClientCredentials manages client credentials grant type requests
func (bs *BearerServer) ClientCredentials(w http.ResponseWriter, r *http.Request) {
	grantType := r.FormValue("grant_type")
	// grant_type client_credentials variables
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	if clientID == "" || clientSecret == "" {
		// get clientID and secret from basic authorization header
		var err error
		clientID, clientSecret, err = GetBasicAuthentication(r)
		if err != nil {
			renderError(w, TokenInvalidClient, "invalid client id or secret", "", http.StatusUnauthorized)
			return
		}
	}
	scope := r.FormValue("scope")
	refreshToken := r.FormValue("refresh_token")
	resp, statusCode := bs.generateTokenResponse(GrantType(grantType), clientID, clientSecret, refreshToken, scope, "", "", r)
	renderJSON(w, resp, GrantType(grantType) == RefreshTokenGrant, statusCode)
}

// AuthorizationCode manages authorization code grant type requests for the phase two of the authorization process
func (bs *BearerServer) AuthorizationCode(w http.ResponseWriter, r *http.Request) {
	grantType := r.FormValue("grant_type")
	// grant_type client_credentials variables
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret") // not mandatory
	code := r.FormValue("code")
	redirectURI := r.FormValue("redirect_uri") // not mandatory
	scope := r.FormValue("scope")              // not mandatory
	if clientID == "" {
		var err error
		clientID, clientSecret, err = GetBasicAuthentication(r)
		if err != nil {
			renderError(w, TokenInvalidClient, "invalid client id or secret", "", http.StatusUnauthorized)
			return
		}
	}
	resp, status := bs.generateTokenResponse(GrantType(grantType), clientID, clientSecret, "", scope, code, redirectURI, r)
	renderJSON(w, resp, GrantType(grantType) == RefreshTokenGrant, status)
}

// Generate token response
func (bs *BearerServer) generateTokenResponse(grantType GrantType, credential string, secret string, refreshToken string, scope string, code string, redirectURI string, r *http.Request) (interface{}, int) {
	var resp *TokenResponse
	switch grantType {
	case PasswordGrant:
		if err := bs.verifier.ValidateUser(credential, secret, scope, r); err != nil {
			return ErrorResponse{Error: TokenInvalidGrant, Description: "invalid username or password", URI: ""}, http.StatusUnauthorized
		}

		token, refresh, err := bs.generateTokens(UserToken, credential, scope, r)
		if err != nil {
			return ErrorResponse{Error: TokenServerError, Description: "token generation failed, check claims: " + err.Error(), URI: ""}, http.StatusInternalServerError
		}

		if err = bs.verifier.StoreTokenID(token.TokenType, credential, token.ID, refresh.ID); err != nil {
			return ErrorResponse{Error: TokenServerError, Description: "storing Token id failed: " + err.Error(), URI: ""}, http.StatusInternalServerError
		}

		if resp, err = bs.cryptTokens(token, refresh, r); err != nil {
			return ErrorResponse{Error: TokenServerError, Description: "token generation failed, check security provider: " + err.Error(), URI: ""}, http.StatusInternalServerError
		}
	case ClientCredentialsGrant:
		if err := bs.verifier.ValidateClient(credential, secret, scope, r); err != nil {
			return ErrorResponse{Error: TokenInvalidGrant, Description: "invalid username or password", URI: ""}, http.StatusUnauthorized
		}

		token, refresh, err := bs.generateTokens(ClientToken, credential, scope, r)
		if err != nil {
			return ErrorResponse{Error: TokenServerError, Description: "token generation failed, check claims: " + err.Error(), URI: ""}, http.StatusInternalServerError
		}

		if err = bs.verifier.StoreTokenID(token.TokenType, credential, token.ID, refresh.ID); err != nil {
			return ErrorResponse{Error: TokenServerError, Description: "storing Token id failed: " + err.Error(), URI: ""}, http.StatusInternalServerError
		}

		if resp, err = bs.cryptTokens(token, refresh, r); err != nil {
			return ErrorResponse{Error: TokenServerError, Description: "token generation failed, check security provider: " + err.Error(), URI: ""}, http.StatusInternalServerError
		}
	case AuthCodeGrant:
		codeVerifier, ok := bs.verifier.(AuthorizationCodeVerifier)
		if !ok {
			return ErrorResponse{Error: TokenUnsupportedGrantType, Description: "grant type is unsupported", URI: ""}, http.StatusBadRequest
		}

		user, err := codeVerifier.ValidateCode(credential, secret, code, redirectURI, r)
		if err != nil {
			return ErrorResponse{Error: TokenInvalidRequest, Description: "invalid username or password", URI: ""}, http.StatusBadRequest
		}

		token, refresh, err := bs.generateTokens(AuthToken, user, scope, r)
		if err != nil {
			return ErrorResponse{Error: TokenServerError, Description: "token generation failed, check claims: " + err.Error(), URI: ""}, http.StatusInternalServerError
		}

		err = bs.verifier.StoreTokenID(token.TokenType, user, token.ID, refresh.ID)
		if err != nil {
			return ErrorResponse{Error: TokenServerError, Description: "storing Token id failed: " + err.Error(), URI: ""}, http.StatusInternalServerError
		}

		if resp, err = bs.cryptTokens(token, refresh, r); err != nil {
			return ErrorResponse{Error: TokenServerError, Description: "token generation failed, check security provider: " + err.Error(), URI: ""}, http.StatusInternalServerError
		}
	case RefreshTokenGrant:
		refresh, err := bs.provider.DecryptRefreshTokens(refreshToken)
		if err != nil || refresh.IsExpired() {
			return ErrorResponse{Error: TokenInvalidRequest, Description: "refresh token is invalid or expired", URI: ""}, http.StatusBadRequest
		}

		if err = bs.verifier.ValidateTokenID(refresh.TokenType, refresh.Credential, refresh.TokenID, refresh.ID); err != nil {
			return ErrorResponse{Error: TokenInvalidRequest, Description: "refresh token is invalid or expired", URI: ""}, http.StatusBadRequest
		}

		token, refresh, err := bs.refreshTokens(refresh.TokenType, refresh.Credential, refresh.Scope, refresh.Claims)
		if err != nil {
			return ErrorResponse{Error: TokenServerError, Description: "token generation failed: " + err.Error(), URI: ""}, http.StatusInternalServerError
		}

		err = bs.verifier.StoreTokenID(token.TokenType, refresh.Credential, token.ID, refresh.ID)
		if err != nil {
			return ErrorResponse{Error: TokenServerError, Description: "storing Token id failed: " + err.Error(), URI: ""}, http.StatusInternalServerError
		}

		if resp, err = bs.cryptTokens(token, refresh, r); err != nil {
			return ErrorResponse{Error: TokenServerError, Description: "token generation failed, check security provider: " + err.Error(), URI: ""}, http.StatusInternalServerError
		}
	default:
		return ErrorResponse{Error: TokenUnsupportedGrantType, Description: "grant type is unsupported", URI: ""}, http.StatusBadRequest
	}

	return resp, http.StatusOK
}

func (bs *BearerServer) refreshTokens(tokenType TokenType, username, scope string, claims Claims) (*Token, *RefreshToken, error) {
	token := &Token{ID: uuid.Must(uuid.NewV4()).String(), Credential: username, ExpiresIn: bs.TokenTTL, CreationDate: time.Now().UTC(), TokenType: tokenType, Scope: scope, Claims: claims}
	refreshToken := &RefreshToken{ID: uuid.Must(uuid.NewV4()).String(), TokenID: token.ID, Credential: username, ExpiresIn: bs.RefreshTokenTTL, CreationDate: time.Now().UTC(), TokenType: tokenType, Scope: scope, Claims: claims}
	return token, refreshToken, nil
}

func (bs *BearerServer) generateTokens(tokenType TokenType, username, scope string, r *http.Request) (*Token, *RefreshToken, error) {
	token := &Token{ID: uuid.Must(uuid.NewV4()).String(), Credential: username, ExpiresIn: bs.TokenTTL, CreationDate: time.Now().UTC(), TokenType: tokenType, Scope: scope}
	var claims Claims
	var err error
	if bs.verifier != nil {
		claims, err = bs.verifier.AddClaims(token.TokenType, username, token.ID, token.Scope, r)
		if err != nil {
			return nil, nil, err
		}
		token.Claims = claims
	}

	refreshToken := &RefreshToken{ID: uuid.Must(uuid.NewV4()).String(), TokenID: token.ID, Credential: username, ExpiresIn: bs.RefreshTokenTTL, CreationDate: time.Now().UTC(), TokenType: tokenType, Scope: scope, Claims: claims}
	return token, refreshToken, nil
}

func (bs *BearerServer) cryptTokens(token *Token, refresh *RefreshToken, r *http.Request) (*TokenResponse, error) {
	cToken, err := bs.provider.CryptToken(token)
	if err != nil {
		return nil, err
	}
	cRefreshToken, err := bs.provider.CryptRefreshToken(refresh)
	if err != nil {
		return nil, err
	}

	tokenResponse := &TokenResponse{Token: cToken, RefreshToken: cRefreshToken, TokenType: BearerToken, ExpiresIn: (int64)(bs.TokenTTL.Seconds()), RefreshTokenExpiresIn: (int64)(bs.RefreshTokenTTL.Seconds())}

	if bs.verifier != nil {
		props, err := bs.verifier.AddProperties(token.TokenType, token.Credential, token.ID, token.Scope, r)
		if err != nil {
			return nil, err
		}
		tokenResponse.Properties = props
	}
	return tokenResponse, nil
}
