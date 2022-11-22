package root

import (
	"os"
	"strings"

	"github.com/Eun/gcal-to-ics/cmd/export"
	"github.com/Eun/gcal-to-ics/cmd/serve"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli"
)

var (
	name        string
	version     string
	commit      string
	date        string
	description string
	author      string
)

var flagLogFile = cli.StringFlag{
	Name:   "logfile",
	Usage:  "Logfile to write to",
	EnvVar: "LOGFILE",
	Value:  "",
}
var flagDebug = cli.BoolFlag{
	Name:   "debug",
	Usage:  "Enable debug log",
	EnvVar: "DEBUG",
}

var flagClientID = cli.StringFlag{
	Name:     "client_id",
	Usage:    "the client id",
	Value:    "",
	EnvVar:   "CLIENT_ID",
	Required: true,
}

var flagClientSecret = cli.StringFlag{
	Name:     "client_secret",
	Usage:    "the client secret",
	Value:    "",
	EnvVar:   "CLIENT_SECRET",
	Required: true,
}

func Run() int {
	var logfile *os.File
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	app := cli.NewApp()
	app.Name = name
	app.Version = strings.TrimSpace(version + " " + commit + " " + date)
	app.Description = description
	app.Author = author
	app.Flags = []cli.Flag{
		flagLogFile,
		flagDebug,
		flagClientID,
		flagClientSecret,
	}
	app.Commands = []cli.Command{
		export.Command,
		serve.Command,
	}
	app.Before = func(context *cli.Context) error {
		loglevel := zerolog.InfoLevel
		if context.Bool(flagDebug.Name) {
			loglevel = zerolog.DebugLevel
		}
		log.Logger = log.Level(loglevel)

		logfilePath := context.String(flagLogFile.Name)
		if logfilePath != "" {
			var err error
			//nolint:gomnd // allow magic number 0666
			logfile, err = os.OpenFile(logfilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_SYNC, 0666)
			if err != nil {
				return errors.Wrap(err, "unable to create logfile")
			}
			log.Logger = log.Output(logfile)
		}
		log.Debug().Str("logfile", logfilePath).Msg("debug log enabled")
		return nil
	}

	defer func() {
		if logfile == nil {
			return
		}
		log.Logger = log.Output(os.Stderr)
		if err := logfile.Close(); err != nil {
			log.Fatal().Err(err).Msg("unable to close logfile")
		}
	}()

	if err := app.Run(os.Args); err != nil {
		log.Error().Err(err).Msg("error during execution")
		return 1
	}
	return 0
}
