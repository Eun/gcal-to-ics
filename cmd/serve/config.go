package serve

import (
	"os"
	"sync"
	"time"

	"github.com/Eun/gcal-to-ics/pkg/gti"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/urfave/cli"
	yaml "gopkg.in/yaml.v3"
)

type CalendarConfig struct {
	AccountEmail    string              `yaml:"account_email" json:"account_email,omitempty"`
	CalendarName    string              `yaml:"calendar_name" json:"calendar_name,omitempty"`
	Formats         []string            `yaml:"formats" json:"formats,omitempty"`
	StartFrom       time.Duration       `yaml:"start_from" json:"start_from,omitempty"`
	EndOn           time.Duration       `yaml:"end_on" json:"end_on,omitempty"`
	HideFields      gti.HideFields      `yaml:"hide_fields" json:"hide_fields"`
	OverwriteFields gti.OverwriteFields `yaml:"overwrite_fields" json:"overwrite_fields"`
}

func readConfig(c *cli.Context, logger *zerolog.Logger) (*sync.Map, error) {
	configFile := c.String(flagConfigFile.Name)
	logger.Debug().Str("config-file", configFile).Msg("reading config")
	f, err := os.Open(configFile)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to open `%s'", configFile)
	}
	defer f.Close()
	var m map[string]*CalendarConfig
	if err := yaml.NewDecoder(f).Decode(&m); err != nil {
		return nil, errors.Wrap(err, "unable to decode config")
	}

	var r sync.Map

	for id, v := range m {
		if v.AccountEmail == "" {
			return nil, errors.New("account_email is missing")
		}
		if v.CalendarName == "" {
			return nil, errors.New("calendar_name is missing")
		}
		if v.EndOn == 0 {
			//nolint: gomnd // default 30 days
			v.EndOn = time.Hour * 24 * 30
		}
		r.Store(id, *v)
	}
	return &r, nil
}
