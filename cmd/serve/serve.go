package serve

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Eun/gcal-to-ics/pkg/gti"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli"
	"golang.org/x/oauth2"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var Command = cli.Command{
	Name:    "serve",
	Aliases: []string{"s"},
	Usage:   "serve calendar files",
	Flags: []cli.Flag{
		flagConfigFile,
		flagBindAddress,
		flagPublicURI,
		flagTokenDir,
		flagCryptSecret,
	},
	Action: action,
}

var flagConfigFile = cli.StringFlag{
	Name:      "config",
	Usage:     "the file to configure the service",
	Value:     "config.yml",
	EnvVar:    "CONFIG_FILE",
	TakesFile: true,
}

var flagBindAddress = cli.StringFlag{
	Name:   "bind-address",
	Usage:  "bind to this address",
	Value:  ":8000",
	EnvVar: "BIND_ADDR",
}

var flagPublicURI = cli.StringFlag{
	Name:   "public-uri",
	Usage:  "public uri this service is reachable",
	Value:  "http://localhost:8000",
	EnvVar: "PUBLIC_URI",
}

var flagTokenDir = cli.StringFlag{
	Name:   "token-dir",
	Usage:  "the directory to store tokens in",
	Value:  "tokens",
	EnvVar: "TOKEN_DIR",
}

var flagCryptSecret = cli.StringFlag{
	Name:   "crypt-secret",
	Usage:  "tokens will be encrypted with this secret",
	Value:  "the cake is a lie",
	EnvVar: "CRYPT_SECRET",
}

func action(c *cli.Context) error {
	logger := log.With().Str("name", c.Command.Name).Logger()

	tokenDir := c.String(flagTokenDir.Name)
	stat, err := os.Stat(tokenDir)
	if err != nil {
		return errors.Wrapf(err, "unable to stat `%s'", tokenDir)
	}
	if !stat.IsDir() {
		return errors.Errorf("`%s' is not a directory", tokenDir)
	}

	cfgMap, err := readConfig(c, &logger)
	if err != nil {
		return errors.Wrap(err, "unable to read config")
	}

	redirectURL, err := url.JoinPath(c.String(flagPublicURI.Name), "auth")
	if err != nil {
		return errors.Wrap(err, "unable to join path")
	}

	var stateMap sync.Map
	type stateEntry struct {
		originalLocation string
		oauthConfig      *oauth2.Config
		accountEmail     string
		validUntil       time.Time
	}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/auth", func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		if state == "" {
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
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, "code is missing")
			return
		}
		v, ok := stateMap.LoadAndDelete(state)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, "state mismatch")
			return
		}

		entry := v.(*stateEntry)
		if entry.validUntil.Before(time.Now()) {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, "expired")
			return
		}

		tkn, err := entry.oauthConfig.Exchange(context.Background(), strings.TrimSpace(code))
		if err != nil {
			logger.Error().Err(err).Msg("unable to exchange token")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "error")
			return
		}
		if !tkn.Valid() {
			logger.Error().Msg("got the token, but its invalid")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "error")
			return
		}

		tokenFile := filepath.Join(tokenDir, hashAccount(entry.accountEmail))

		if err := writeTokenFile(c.String(flagCryptSecret.Name), tokenFile, tkn); err != nil {
			logger.Error().Msg("unable to write token")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "error")
			return
		}

		http.Redirect(w, r, entry.originalLocation, http.StatusTemporaryRedirect)
		_, _ = io.WriteString(w, "authorized, you can close this window.")
	})
	r.Get("/{id:[a-zA-Z-0-9]+}.{format}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		format := chi.URLParam(r, "format")
		if id == "" || format == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "malformed request")
			return
		}

		cfg, ok := cfgMap.Load(id)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, "not found")
			return
		}
		calendarConfig, ok := cfg.(CalendarConfig)
		if !ok {
			logger.Error().Msgf("calendarConfig is not %T it is %T", CalendarConfig{}, calendarConfig)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "internal server error")
			return
		}

		wantedFormatIsAllowed := false
		for _, f := range calendarConfig.Formats {
			if format == f {
				wantedFormatIsAllowed = true
				break
			}
		}

		if !wantedFormatIsAllowed {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "wanted format is not allowed")
			return
		}

		tokenFile := filepath.Join(tokenDir, hashAccount(calendarConfig.AccountEmail))

		// get the client
		oauthConfig := createOauthConfig(redirectURL, c.GlobalString("client_id"), c.GlobalString("client_secret"))

		client, err := getAuthenticatedClient(&logger, c.String(flagCryptSecret.Name), tokenFile, oauthConfig)
		if err != nil {
			logger.Error().Err(err).Msg("unable to get authenticated client")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "internal server error")
			return
		}
		if client == nil {
			logger.Debug().Msg("no token available, redirect to authorization")
			state := uuid.New().String()
			stateMap.Store(state, &stateEntry{
				originalLocation: r.RequestURI,
				oauthConfig:      oauthConfig,
				accountEmail:     calendarConfig.AccountEmail,
				//nolint: gomnd // default timeout is 5 mins
				validUntil: time.Now().Add(time.Minute * 5),
			})
			http.Redirect(w, r, oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline), http.StatusTemporaryRedirect)
			return
		}

		var buf bytes.Buffer
		err = gti.Export(&gti.Config{
			Format:          format,
			AccountEmail:    calendarConfig.AccountEmail,
			Logger:          &logger,
			StartFrom:       time.Now().Add(-calendarConfig.StartFrom),
			EndOn:           time.Now().Add(calendarConfig.EndOn),
			CalendarName:    calendarConfig.CalendarName,
			Writer:          &buf,
			Client:          client,
			Version:         c.App.Version,
			HideFields:      calendarConfig.HideFields,
			OverwriteFields: calendarConfig.OverwriteFields,
		})

		if err != nil {
			logger.Error().Err(err).Msg("export failed")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "internal server error")
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, &buf)
		buf.Reset()
	})
	logger.Debug().
		Str("address", c.String(flagBindAddress.Name)).
		Str("public_uri", c.String(flagPublicURI.Name)).
		Msg("listening")

	server := http.Server{
		Addr:              c.String(flagBindAddress.Name),
		Handler:           r,
		ReadTimeout:       time.Second,
		WriteTimeout:      time.Second,
		IdleTimeout:       30 * time.Second, //nolint: gomnd // set timeout
		ReadHeaderTimeout: 2 * time.Second,  //nolint: gomnd // set timeout
	}

	return server.ListenAndServe()
}
