package export

import (
	"os"
	"time"

	"github.com/Eun/gcal-to-ics/pkg/gti"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli"
)

var Command = cli.Command{
	Name:    "export",
	Aliases: []string{"e"},
	Usage:   "export calendar to specific file",
	Flags: []cli.Flag{
		flagTokenFile,
		flagAuthBindAddress,
		flagAccount,
		flagCalendar,
		flagFormat,
		flagStartFrom,
		flagEndOn,
		flagOutput,

		flagHideUID,
		flagHideOrganizer,
		flagHideAttendees,
		flagHideVisibility,
		flagHideDescription,
		flagHideLocation,
		flagHideConference,
		flagHideTransparency,
		flagHideStatus,

		flagOverwriteCalendarName,
		flagOverwriteOrganizer,
		flagOverwriteVisibility,
		flagOverwriteDescription,
		flagOverwriteLocation,
		flagOverwriteConference,
		flagOverwriteTransparency,
		flagOverwriteStatus,
	},
	Action: action,
}

var flagTokenFile = cli.StringFlag{
	Name:  "tokenfile",
	Usage: "the file where the token will be stored",
	Value: "token.json",
}

var flagAuthBindAddress = cli.StringFlag{
	Name:  "auth-bind-address",
	Usage: "bind to this address for the google authentication",
	Value: "127.0.0.1:8000",
}

var flagAccount = cli.StringFlag{
	Name:     "account",
	Usage:    "google account to use in the format <user@domain.com>",
	Required: true,
}

var flagCalendar = cli.StringFlag{
	Name:     "calendar",
	Usage:    "which calendar to use",
	Required: true,
}

var flagFormat = cli.StringFlag{
	Name:  "format",
	Usage: "which format to export to",
	Value: "ics",
}

var flagStartFrom = cli.StringFlag{
	Name:  "start-from",
	Usage: "from which time to start export event from",
	Value: time.Now().Format(time.RFC3339),
}

var flagEndOn = cli.StringFlag{
	Name:  "end-on",
	Usage: "on which time to end export events",
	Value: time.Now().AddDate(0, 1, 0).Format(time.RFC3339),
}

var flagOutput = cli.StringFlag{
	Name:  "output",
	Usage: "where to export to",
	Value: "-",
}

var flagHideUID = cli.BoolFlag{
	Name:  "hide.uid",
	Usage: "whether or not to hide uid",
}
var flagHideOrganizer = cli.BoolFlag{
	Name:  "hide.organizer",
	Usage: "whether or not to hide organizer",
}
var flagHideAttendees = cli.BoolFlag{
	Name:  "hide.attendees",
	Usage: "whether or not to hide attendees",
}
var flagHideVisibility = cli.BoolFlag{
	Name:  "hide.visibility",
	Usage: "whether or not to hide visibility",
}
var flagHideDescription = cli.BoolFlag{
	Name:  "hide.description",
	Usage: "whether or not to hide description",
}
var flagHideLocation = cli.BoolFlag{
	Name:  "hide.location",
	Usage: "whether or not to hide location",
}
var flagHideConference = cli.BoolFlag{
	Name:  "hide.conference",
	Usage: "whether or not to hide conference",
}
var flagHideTransparency = cli.BoolFlag{
	Name:  "hide.transparency",
	Usage: "whether or not to hide transparency",
}
var flagHideStatus = cli.BoolFlag{
	Name:  "hide.status",
	Usage: "whether or not to hide status",
}

var flagOverwriteCalendarName = cli.StringFlag{
	Name:  "overwrite.calendar-name",
	Usage: "overwrite CalendarName with the specified value",
}
var flagOverwriteOrganizer = cli.StringFlag{
	Name:  "overwrite.organizer",
	Usage: "overwrite Organizer with the specified value",
}
var flagOverwriteVisibility = cli.StringFlag{
	Name:  "overwrite.visibility",
	Usage: "overwrite Visibility with the specified value",
}
var flagOverwriteDescription = cli.StringFlag{
	Name:  "overwrite.description",
	Usage: "overwrite Description with the specified value",
}
var flagOverwriteLocation = cli.StringFlag{
	Name:  "overwrite.location",
	Usage: "overwrite Location with the specified value",
}
var flagOverwriteConference = cli.StringFlag{
	Name:  "overwrite.conference",
	Usage: "overwrite Conference with the specified value",
}
var flagOverwriteTransparency = cli.StringFlag{
	Name:  "overwrite.transparency",
	Usage: "overwrite Transparency with the specified value",
}
var flagOverwriteStatus = cli.StringFlag{
	Name:  "overwrite.status",
	Usage: "overwrite Status with the specified value",
}

func action(c *cli.Context) error {
	logger := log.With().Str("name", c.Command.Name).Logger()

	startFrom, err := time.Parse(time.RFC3339, c.String(flagStartFrom.Name))
	if err != nil {
		return errors.Wrapf(err, "unable to parse start-from `%s'", c.String(flagStartFrom.Name))
	}
	endOn, err := time.Parse(time.RFC3339, c.String(flagEndOn.Name))
	if err != nil {
		return errors.Wrapf(err, "unable to parse end-on `%s'", c.String(flagEndOn.Name))
	}
	outputFile := c.String(flagOutput.Name)

	if outputFile == "" || outputFile == "-" {
		c.App.Writer = os.Stdout
	} else {
		var f *os.File
		f, err = os.Create(outputFile)
		if err != nil {
			return errors.Wrapf(err, "unable to open outputfile `%s'", outputFile)
		}
		defer f.Close()
		c.App.Writer = f
	}

	client, err := getAuthenticatedClient(
		&logger,
		c.String(flagAuthBindAddress.Name),
		c.String(flagTokenFile.Name),
		c.GlobalString("client_id"),
		c.GlobalString("client_secret"),
	)
	if err != nil {
		return errors.Wrap(err, "unable to get authenticated client")
	}

	return gti.Export(&gti.Config{
		Format:       c.String(flagFormat.Name),
		AccountEmail: c.String(flagAccount.Name),
		Logger:       &logger,
		StartFrom:    startFrom,
		EndOn:        endOn,
		CalendarName: c.String(flagCalendar.Name),
		Writer:       c.App.Writer,
		Client:       client,
		Version:      c.App.Version,
		HideFields: gti.HideFields{
			UID:          c.Bool(flagHideUID.Name),
			Organizer:    c.Bool(flagHideOrganizer.Name),
			Attendees:    c.Bool(flagHideAttendees.Name),
			Visibility:   c.Bool(flagHideVisibility.Name),
			Description:  c.Bool(flagHideDescription.Name),
			Location:     c.Bool(flagHideLocation.Name),
			Conference:   c.Bool(flagHideConference.Name),
			Transparency: c.Bool(flagHideTransparency.Name),
			Status:       c.Bool(flagHideStatus.Name),
		},
		OverwriteFields: gti.OverwriteFields{
			CalendarName: c.String(flagOverwriteCalendarName.Name),
			Organizer:    c.String(flagOverwriteOrganizer.Name),
			Visibility:   c.String(flagOverwriteVisibility.Name),
			Description:  c.String(flagOverwriteDescription.Name),
			Location:     c.String(flagOverwriteLocation.Name),
			Conference:   c.String(flagOverwriteConference.Name),
			Transparency: c.String(flagOverwriteTransparency.Name),
			Status:       c.String(flagOverwriteStatus.Name),
		},
	})
}
