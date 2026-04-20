package launchagent

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"

	"github.com/emkaytec/forge/pkg/schema"
)

// livePlist captures the subset of plist fields the launch-agent
// handler cares about. Fields outside this shape are ignored per
// ADR-like guidance that unknown live-state fields do not drive
// drift (MK-7 keeps the schema deliberately narrow).
type livePlist struct {
	Label                 string
	ProgramArguments      []string
	RunAtLoad             bool
	StartInterval         int
	StartIntervalSet      bool
	CalendarHour          int
	CalendarMinute        int
	StartCalendarPresent  bool
}

// parseLivePlist decodes a launchd plist XML document into a
// minimal livePlist. Missing keys return zero values.
func parseLivePlist(data []byte) (*livePlist, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false

	// Seek to <plist> -> <dict>.
	if err := seekStart(dec, "plist"); err != nil {
		return nil, fmt.Errorf("launchagent: %w", err)
	}
	if err := seekStart(dec, "dict"); err != nil {
		return nil, fmt.Errorf("launchagent: %w", err)
	}

	plist, err := decodeDict(dec)
	if err != nil {
		return nil, fmt.Errorf("launchagent: decode plist: %w", err)
	}

	return plist, nil
}

func seekStart(dec *xml.Decoder, name string) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("expected <%s>: %w", name, err)
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == name {
			return nil
		}
	}
}

func decodeDict(dec *xml.Decoder) (*livePlist, error) {
	out := &livePlist{}

	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}

		switch t := tok.(type) {
		case xml.EndElement:
			if t.Name.Local == "dict" {
				return out, nil
			}
		case xml.StartElement:
			if t.Name.Local != "key" {
				if err := dec.Skip(); err != nil {
					return nil, err
				}
				continue
			}

			var key string
			if err := dec.DecodeElement(&key, &t); err != nil {
				return nil, err
			}

			if err := readValue(dec, key, out); err != nil {
				return nil, err
			}
		}
	}
}

func readValue(dec *xml.Decoder, key string, out *livePlist) error {
	start, err := nextStart(dec)
	if err != nil {
		return err
	}

	switch key {
	case "Label":
		var v string
		if err := dec.DecodeElement(&v, &start); err != nil {
			return err
		}
		out.Label = v
	case "ProgramArguments":
		args, err := decodeStringArray(dec, &start)
		if err != nil {
			return err
		}
		out.ProgramArguments = args
	case "RunAtLoad":
		out.RunAtLoad = start.Name.Local == "true"
		if err := dec.Skip(); err != nil {
			return err
		}
	case "StartInterval":
		var v string
		if err := dec.DecodeElement(&v, &start); err != nil {
			return err
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("StartInterval: %w", err)
		}
		out.StartInterval = n
		out.StartIntervalSet = true
	case "StartCalendarInterval":
		if start.Name.Local != "dict" {
			return fmt.Errorf("StartCalendarInterval expected <dict>, got <%s>", start.Name.Local)
		}
		hour, minute, err := decodeCalendar(dec)
		if err != nil {
			return err
		}
		out.CalendarHour = hour
		out.CalendarMinute = minute
		out.StartCalendarPresent = true
	default:
		if err := dec.Skip(); err != nil {
			return err
		}
	}

	return nil
}

func decodeStringArray(dec *xml.Decoder, start *xml.StartElement) ([]string, error) {
	if start.Name.Local != "array" {
		// Single value, not an array.
		var v string
		if err := dec.DecodeElement(&v, start); err != nil {
			return nil, err
		}
		return []string{v}, nil
	}

	var out []string
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.EndElement:
			if t.Name.Local == "array" {
				return out, nil
			}
		case xml.StartElement:
			if t.Name.Local == "string" {
				var v string
				if err := dec.DecodeElement(&v, &t); err != nil {
					return nil, err
				}
				out = append(out, v)
				continue
			}
			if err := dec.Skip(); err != nil {
				return nil, err
			}
		}
	}
}

func decodeCalendar(dec *xml.Decoder) (hour, minute int, err error) {
	for {
		tok, tokErr := dec.Token()
		if tokErr != nil {
			return 0, 0, tokErr
		}

		switch t := tok.(type) {
		case xml.EndElement:
			if t.Name.Local == "dict" {
				return hour, minute, nil
			}
		case xml.StartElement:
			if t.Name.Local != "key" {
				if skipErr := dec.Skip(); skipErr != nil {
					return 0, 0, skipErr
				}
				continue
			}

			var key string
			if decErr := dec.DecodeElement(&key, &t); decErr != nil {
				return 0, 0, decErr
			}

			valueStart, startErr := nextStart(dec)
			if startErr != nil {
				return 0, 0, startErr
			}

			var raw string
			if decErr := dec.DecodeElement(&raw, &valueStart); decErr != nil {
				return 0, 0, decErr
			}

			n, convErr := strconv.Atoi(raw)
			if convErr != nil {
				return 0, 0, fmt.Errorf("calendar %s: %w", key, convErr)
			}

			switch key {
			case "Hour":
				hour = n
			case "Minute":
				minute = n
			}
		}
	}
}

func nextStart(dec *xml.Decoder) (xml.StartElement, error) {
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return xml.StartElement{}, fmt.Errorf("unexpected EOF")
			}
			return xml.StartElement{}, err
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se, nil
		}
	}
}

// toSpec converts a livePlist to a LaunchAgentSpec so it can be
// diffed against the desired spec.
func (p *livePlist) toSpec() *schema.LaunchAgentSpec {
	spec := &schema.LaunchAgentSpec{
		Label:     p.Label,
		Command:   extractCommand(p.ProgramArguments),
		RunAtLoad: p.RunAtLoad,
	}

	switch {
	case p.StartIntervalSet:
		spec.Schedule = schema.LaunchAgentSchedule{
			Type:            schema.ScheduleTypeInterval,
			IntervalSeconds: p.StartInterval,
		}
	case p.StartCalendarPresent:
		spec.Schedule = schema.LaunchAgentSchedule{
			Type:   schema.ScheduleTypeCalendar,
			Hour:   p.CalendarHour,
			Minute: p.CalendarMinute,
		}
	}

	return spec
}

// extractCommand reverses the ProgramArguments encoding used by
// renderPlist: ["/bin/sh", "-c", <command>]. If the live plist does
// not match that shape, return the joined arguments so callers see
// the mismatch as drift rather than silent loss.
func extractCommand(args []string) string {
	if len(args) == 3 && args[0] == "/bin/sh" && args[1] == "-c" {
		return args[2]
	}

	if len(args) == 0 {
		return ""
	}

	out := args[0]
	for _, a := range args[1:] {
		out += " " + a
	}
	return out
}
