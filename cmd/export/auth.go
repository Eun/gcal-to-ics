package export

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

func getAuthenticatedClient(logger *zerolog.Logger, authAddress, tokenFile, clientID, clientSecret string) (*http.Client, error) {
	var tokenBuf []byte
	writeTokenFile := false
	if tokenFile != "" {
		var err error
		tokenBuf, err = os.ReadFile(tokenFile)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, errors.Wrapf(err, "unable to open file `%s'", tokenFile)
			}
			tokenBuf = nil
			writeTokenFile = true
		}
	}

	oauthConfig := createOauthConfig(authAddress, clientID, clientSecret)

	if len(tokenBuf) == 0 {
		var err error
		tokenBuf, err = fetchNewToken(oauthConfig, authAddress)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		writeTokenFile = true
	}
	oauth2Token, err := decodeOauthToken(logger, tokenBuf)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	tokenSource := oauthConfig.TokenSource(context.Background(), oauth2Token)
	updatedOauth2Token, err := tokenSource.Token()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get token")
	}

	client := oauth2.NewClient(context.Background(), tokenSource)

	if updatedOauth2Token.AccessToken != oauth2Token.AccessToken {
		writeTokenFile = true
	}

	if writeTokenFile && tokenFile != "" {
		buf, err := json.Marshal(updatedOauth2Token)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to encode token")
		}
		//nolint: gomnd // default permissions
		if err := os.WriteFile(tokenFile, buf, 0600); err != nil {
			return nil, errors.Wrapf(err, "unable to write file `%s'", tokenFile)
		}
	}

	return client, nil
}

func fetchNewToken(oauthConfig *oauth2.Config, authAddress string) ([]byte, error) {
	state := uuid.New().String()
	codeChan := make(chan string)
	errChan := make(chan error)
	var httpServer http.Server
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("state") != state {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprint(w, "state mismatch")
				return
			}
			if s := r.URL.Query().Get("error"); s != "" {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, "error: %s", s)
				return
			}
			code := r.URL.Query().Get("code")
			if code == "" {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, "code is missing")
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "authorized, you can close this window.")
			codeChan <- code
		})
		httpServer.Addr = authAddress
		httpServer.Handler = mux
		errChan <- httpServer.ListenAndServe()
	}()

	authURL := oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)

	fmt.Println("Please open", authURL)
	fmt.Println("Waiting for authorization...")

	var code string
	select {
	case c := <-codeChan:
		code = c
	case err := <-errChan:
		return nil, errors.Wrapf(err, "unable to listen on http server")
	}
	_ = httpServer.Close()

	tkn, err := oauthConfig.Exchange(context.Background(), strings.TrimSpace(code))
	if err != nil {
		return nil, errors.Wrap(err, "unable to exchange token")
	}
	if !tkn.Valid() {
		return nil, errors.New("got the token, but its invalid")
	}
	tokenBuf, err := json.Marshal(tkn)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to encode token")
	}
	return tokenBuf, nil
}

func createOauthConfig(authAddress, clientID, clientSecret string) *oauth2.Config {
	return &oauth2.Config{
		Scopes: []string{
			"https://www.googleapis.com/auth/calendar.readonly",
			"https://www.googleapis.com/auth/calendar.events.readonly",
		},
		RedirectURL: "http://" + authAddress,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://accounts.google.com/o/oauth2/token",
		},
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
}

func decodeOauthToken(logger *zerolog.Logger, buf []byte) (*oauth2.Token, error) {
	logger.Debug().Msg("json decoding token")
	var token oauth2.Token
	if err := json.Unmarshal(buf, &token); err != nil {
		return nil, errors.New("unable to decode token")
	}

	logger.Debug().Msg("oauth2 token decoded")
	return &token, nil
}
