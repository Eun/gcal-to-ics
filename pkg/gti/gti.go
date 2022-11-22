package gti

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type Config struct {
	Format          string
	AccountEmail    string
	Logger          *zerolog.Logger
	StartFrom       time.Time
	EndOn           time.Time
	CalendarName    string
	Writer          io.Writer
	Client          *http.Client
	Version         string
	HideFields      HideFields
	OverwriteFields OverwriteFields
}

type HideFields struct {
	UID          bool `yaml:"uid" json:"uid,omitempty"`
	Organizer    bool `yaml:"organizer" json:"organizer,omitempty"`
	Attendees    bool `yaml:"attendees" json:"attendees,omitempty"`
	Visibility   bool `yaml:"visibility" json:"visibility,omitempty"`
	Description  bool `yaml:"description" json:"description,omitempty"`
	Location     bool `yaml:"location" json:"location,omitempty"`
	Conference   bool `yaml:"conference" json:"conference,omitempty"`
	Transparency bool `yaml:"transparency" json:"transparency,omitempty"`
	Status       bool `yaml:"status" json:"status,omitempty"`
}

type OverwriteFields struct {
	CalendarName string `yaml:"calendar_name" json:"calendar_name,omitempty"`
	Organizer    string `yaml:"organizer" json:"organizer,omitempty"`
	Visibility   string `yaml:"visibility" json:"visibility,omitempty"`
	Description  string `yaml:"description" json:"description,omitempty"`
	Location     string `yaml:"location" json:"location,omitempty"`
	Conference   string `yaml:"conference" json:"conference,omitempty"`
	Transparency string `yaml:"transparency" json:"transparency,omitempty"`
	Status       string `yaml:"status" json:"status,omitempty"`
}

func Export(config *Config) error {
	if config == nil {
		return errors.New("config cannot be nil")
	}
	if config.Format != "ics" {
		return errors.Errorf("format `%s' is not supported", config.Format)
	}

	config.Logger.Debug().Msg("getting calendar service")
	service, err := calendar.NewService(context.Background(), option.WithHTTPClient(config.Client))
	if err != nil {
		return errors.Wrap(err, "unable to create calendar service")
	}

	config.Logger.Debug().Str("calendar", config.CalendarName).Msg("finding calendar id")
	calendarID, err := findCalendarID(service, config.CalendarName)
	if err != nil {
		return errors.Wrapf(err, "unable to find calendar id for `%s'", config.CalendarName)
	}
	if calendarID == "" {
		return errors.Errorf("no such calendar `%s'", config.CalendarName)
	}

	config.Logger.Debug().
		Str("calendar", config.CalendarName).
		Str("calendar_id", calendarID).
		Msg("found calendar id")

	return writeEvents(service, calendarID, config)
}

func findCalendarID(service *calendar.Service, calendarName string) (string, error) {
	var nextPageToken string
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		call := service.CalendarList.List().
			MaxResults(maxCalendarsToFetchPerAPICall).
			ShowHidden(true).
			Context(ctx)
		if nextPageToken != "" {
			call.PageToken(nextPageToken)
		}

		list, err := call.Do()
		cancel()
		if err != nil {
			return "", errors.Wrap(err, "unable to list calendars")
		}

		if list == nil {
			return "", errors.New("list is nil")
		}

		for i := range list.Items {
			if list.Items[i].Deleted {
				continue
			}
			if list.Items[i].Summary == calendarName {
				return list.Items[i].Id, nil
			}
		}

		if list.NextPageToken == "" {
			break
		}
		nextPageToken = list.NextPageToken
	}
	return "", nil
}

const icalTimestampFormatUtc = "20060102T150405Z"
const icalDateFormatUtc = "20060102"
const googleDateFormat = "2006-01-02"
const maxCalendarsToFetchPerAPICall = 100
const maxEventsToFetchPerAPICall = 100

var textEscaper = strings.NewReplacer(
	`\`, `\\`,
	"\n", `\n`,
	`;`, `\;`,
	`,`, `\,`,
)

func toText(s string) string {
	// Some special characters for iCalendar format should be escaped while
	// setting a value of a property with a TEXT type.
	return textEscaper.Replace(s)
}

func writeHeader(config *Config, cal *calendar.Calendar) error {
	// write header
	for _, s := range []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//gcal-to-ics//gcal-to-ics-" + config.Version + "//EN",
		"CALSCALE:GREGORIAN",
		"METHOD:PUBLISH",
		"X-WR-TIMEZONE:" + cal.TimeZone,
	} {
		if _, err := fmt.Fprint(config.Writer, s, "\n"); err != nil {
			return errors.WithStack(err)
		}
	}

	if config.OverwriteFields.CalendarName != "" {
		if _, err := fmt.Fprint(config.Writer, "X-WR-CALNAME:", cal.Summary, "\n"); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func writeTrailer(config *Config) error {
	// write header
	for _, s := range []string{
		"END:VCALENDAR",
	} {
		if _, err := fmt.Fprint(config.Writer, s, "\n"); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func writeEvent(config *Config, ev *calendar.Event) error {
	fmt.Fprintf(config.Writer, "BEGIN:VEVENT\n")

	if !config.HideFields.UID {
		fmt.Fprintf(config.Writer, "UID:%s\n", ev.ICalUID)
	}

	if err := writeEventTime(config, ev); err != nil {
		return errors.WithStack(err)
	}

	fmt.Fprintf(config.Writer, "SUMMARY:%s\n", toText(ev.Summary))
	if !config.HideFields.Description {
		if config.OverwriteFields.Description != "" {
			ev.Description = config.OverwriteFields.Description
		}
		if ev.Description != "" {
			fmt.Fprintf(config.Writer, "DESCRIPTION:%s\n", toText(ev.Description))
		}
	}

	if !config.HideFields.Transparency {
		if config.OverwriteFields.Transparency != "" {
			ev.Transparency = config.OverwriteFields.Transparency
		}
		if strings.EqualFold(ev.Transparency, "TRANSPARENT") {
			fmt.Fprint(config.Writer, "TRANSP:TRANSPARENT\n")
		} else {
			fmt.Fprint(config.Writer, "TRANSP:OPAQUE\n")
		}
	}

	if !config.HideFields.Location {
		if config.OverwriteFields.Location != "" {
			ev.Location = config.OverwriteFields.Location
		}
		if ev.Location != "" {
			fmt.Fprintf(config.Writer, "LOCATION:%s\n", toText(ev.Location))
		}
	}

	if !config.HideFields.Visibility {
		if config.OverwriteFields.Visibility != "" {
			ev.Visibility = config.OverwriteFields.Visibility
		}
		switch strings.ToUpper(ev.Visibility) {
		case "PUBLIC":
			fmt.Fprintf(config.Writer, "CLASS:%s\n", toText("PUBLIC"))
		case "PRIVATE":
			fmt.Fprintf(config.Writer, "CLASS:%s\n", toText("PRIVATE"))
		}
	}

	if !config.HideFields.Conference {
		if config.OverwriteFields.Conference != "" {
			fmt.Fprintf(config.Writer, "X-GOOGLE-CONFERENCE:%s\n", toText(config.OverwriteFields.Conference))
		} else if ev.ConferenceData != nil && len(ev.ConferenceData.EntryPoints) > 0 {
			for _, point := range ev.ConferenceData.EntryPoints {
				if point.Uri == "" {
					continue
				}
				fmt.Fprintf(config.Writer, "X-GOOGLE-CONFERENCE:%s\n", toText(point.Uri))
				break
			}
		}
	}

	if !config.HideFields.Organizer {
		if config.OverwriteFields.Organizer != "" {
			fmt.Fprintf(config.Writer, "ORGANIZER:%s\n", toText(config.OverwriteFields.Organizer))
		} else if ev.Organizer != nil && ev.Organizer.Email != "" {
			displayName := ev.Organizer.DisplayName
			if displayName == "" {
				displayName = ev.Organizer.Email
			}
			fmt.Fprintf(config.Writer, "ORGANIZER:CN=%s:mailto:%s\n", displayName, ev.Organizer.Email)
		}
	}

	if !config.HideFields.Attendees {
		for _, attendee := range ev.Attendees {
			if attendee == nil || attendee.Email == "" {
				continue
			}

			role := "REQ-PARTICIPANT"
			if attendee.Optional {
				role = "OPT-PARTICIPANT"
			}
			var partStat string
			switch attendee.ResponseStatus {
			case "needsAction":
				partStat = "NEEDS-ACTION"
			case "declined":
				partStat = "DECLINED"
			case "tentative":
				partStat = "TENTATIVE"
			case "accepted":
				partStat = "ACCEPTED"
			}

			// overwrite status if we are an attendee
			if strings.EqualFold(attendee.Email, config.AccountEmail) {
				ev.Status = attendee.ResponseStatus
			}

			displayName := attendee.DisplayName
			if displayName == "" {
				displayName = attendee.Email
			}

			fmt.Fprintf(config.Writer, "ATTENDEE;ROLE=%s;PARTSTAT=%s;CN=%s:mailto:%s",
				role,
				partStat,
				displayName,
				attendee.Email,
			)
		}
	}

	if !config.HideFields.Status {
		if config.OverwriteFields.Status != "" {
			ev.Status = config.OverwriteFields.Status
		}

		switch strings.ToUpper(ev.Status) {
		case "TENTATIVE":
			fmt.Fprint(config.Writer, "STATUS:TENTATIVE\n")
		case "CANCELLED": //nolint: misspell // not a misspell
			fmt.Fprint(config.Writer, "STATUS:CANCELLED\n") //nolint: misspell // not a misspell
		case "CONFIRMED":
			fmt.Fprint(config.Writer, "STATUS:CONFIRMED\n")
		}
	}

	created, err := time.Parse(time.RFC3339, ev.Created)
	if err == nil {
		fmt.Fprintf(config.Writer, "DTSTAMP:%s\n", created.UTC().Format(icalTimestampFormatUtc))
		fmt.Fprintf(config.Writer, "CREATED:%s\n", created.UTC().Format(icalTimestampFormatUtc))
	}
	updated, err := time.Parse(time.RFC3339, ev.Updated)
	if err == nil {
		fmt.Fprintf(config.Writer, "LAST-MODIFIED:%s\n", updated.UTC().Format(icalTimestampFormatUtc))
	}
	fmt.Fprintf(config.Writer, "END:VEVENT\n")
	return nil
}

func writeEventTime(config *Config, ev *calendar.Event) error {
	if ev.Start.Date != "" && ev.End.Date != "" {
		// all day event
		return writeAllDayEventTime(config, ev)
	}
	startTime, err := time.Parse(time.RFC3339, ev.Start.DateTime)
	if err != nil {
		startTime = time.Time{}
	}
	endTime, err := time.Parse(time.RFC3339, ev.End.DateTime)
	if err != nil {
		endTime = time.Time{}
	}

	if !startTime.IsZero() && !endTime.IsZero() {
		_, err = fmt.Fprintf(config.Writer, "DTSTART:%s\n", startTime.UTC().Format(icalTimestampFormatUtc))
		if err != nil {
			return errors.WithStack(err)
		}
		_, err = fmt.Fprintf(config.Writer, "DTEND:%s\n", endTime.UTC().Format(icalTimestampFormatUtc))
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func writeAllDayEventTime(config *Config, ev *calendar.Event) error {
	startTime, err := time.Parse(googleDateFormat, ev.Start.Date)
	if err != nil {
		startTime = time.Time{}
	}
	endTime, err := time.Parse(googleDateFormat, ev.End.Date)
	if err != nil {
		endTime = time.Time{}
	}

	if startTime.IsZero() || endTime.IsZero() {
		return nil
	}
	_, err = fmt.Fprintf(config.Writer, "DTSTART;VALUE=DATE:%s\n", startTime.UTC().Format(icalDateFormatUtc))
	if err != nil {
		return errors.WithStack(err)
	}
	_, err = fmt.Fprintf(config.Writer, "DTEND;VALUE=DATE:%s\n", endTime.UTC().Format(icalDateFormatUtc))
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func writeEvents(service *calendar.Service, calendarID string, config *Config) error {
	// get some details about the calendar
	config.Logger.Debug().Str("calendar_id", calendarID).Msg("getting calendar details")
	cal, err := service.Calendars.Get(calendarID).Do()
	if err != nil {
		return errors.Wrapf(err, "unable to get details for calendar `%s'", calendarID)
	}
	if err := writeHeader(config, cal); err != nil {
		return errors.Wrapf(err, "unable to write header")
	}

	var nextPageToken string
	var totalEvents int
	for {
		config.Logger.Debug().
			Str("calendar_id", calendarID).
			Time("start_from", config.StartFrom).
			Time("end_on", config.EndOn).
			Str("next_page_token", nextPageToken).
			Msg("finding events")

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		call := service.Events.List(calendarID).
			MaxResults(maxEventsToFetchPerAPICall).
			ShowDeleted(false).
			TimeMin(config.StartFrom.Format(time.RFC3339)).
			TimeMax(config.EndOn.Format(time.RFC3339)).
			Context(ctx)
		if nextPageToken != "" {
			call.PageToken(nextPageToken)
		}

		list, err := call.Do()
		cancel()
		if err != nil {
			return errors.Wrap(err, "unable to list events")
		}

		if list == nil {
			return errors.New("list is nil")
		}

		config.Logger.Debug().Msgf("found %d items", len(list.Items))

		for _, ev := range list.Items {
			if ev.Id == "" || ev.Summary == "" || ev.Start == nil || ev.End == nil {
				continue
			}
			if err := writeEvent(config, ev); err != nil {
				return errors.Wrap(err, "unable to write event")
			}
		}
		totalEvents += len(list.Items)

		if list.NextPageToken == "" {
			break
		}
		nextPageToken = list.NextPageToken
	}

	if err := writeTrailer(config); err != nil {
		return errors.Wrapf(err, "unable to write trailer")
	}

	config.Logger.Debug().
		Str("calendar_id", calendarID).
		Msgf("written %d events", totalEvents)
	return nil
}
