package serve

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/rs/zerolog"

	"github.com/pkg/errors"
	"golang.org/x/crypto/nacl/secretbox"
	"golang.org/x/oauth2"
)

func getAuthenticatedClient(logger *zerolog.Logger, key, tokenFile string, oauthConfig *oauth2.Config) (*http.Client, error) {
	updateTokenFile := false
	tokenBuf, err := decryptFile(key, tokenFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "unable to open file `%s'", tokenFile)
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
		updateTokenFile = true
	}

	if updateTokenFile && tokenFile != "" {
		if err := writeTokenFile(key, tokenFile, updatedOauth2Token); err != nil {
			return nil, err
		}
	}

	return client, nil
}

func writeTokenFile(key, tokenFile string, tkn *oauth2.Token) error {
	buf, err := json.Marshal(tkn)
	if err != nil {
		return errors.Wrapf(err, "unable to encode token")
	}

	//nolint: gomnd // default permissions
	if err := os.WriteFile(tokenFile, crypt(key, buf), 0600); err != nil {
		return errors.Wrapf(err, "unable to write file `%s'", tokenFile)
	}
	return nil
}

func createOauthConfig(redirectURL, clientID, clientSecret string) *oauth2.Config {
	return &oauth2.Config{
		Scopes: []string{
			"https://www.googleapis.com/auth/calendar.readonly",
			"https://www.googleapis.com/auth/calendar.events.readonly",
		},
		RedirectURL: redirectURL,
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

const nonceSize = 24

func crypt(key string, in []byte) []byte {
	k := sha256.Sum256([]byte(key))

	var nonce [nonceSize]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		panic(err)
	}

	return secretbox.Seal(nonce[:], in, &nonce, &k)
}

func decrypt(key string, in []byte) ([]byte, bool) {
	if len(in) < nonceSize {
		return nil, false
	}
	k := sha256.Sum256([]byte(key))

	var nonce [nonceSize]byte
	copy(nonce[:], in[:nonceSize])
	return secretbox.Open(nil, in[nonceSize:], &nonce, &k)
}

func hashAccount(in string) string {
	v := sha256.Sum256([]byte(in))
	return hex.EncodeToString(v[:])
}

func decryptFile(key, name string) ([]byte, error) {
	buf, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	plain, ok := decrypt(key, buf)
	if !ok {
		return nil, errors.New("unable to decrypt")
	}
	return plain, nil
}
