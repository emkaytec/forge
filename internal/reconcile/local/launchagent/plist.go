package launchagent

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"

	"github.com/emkaytec/forge/pkg/schema"
)

const plistDoctype = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
`

// renderPlist encodes spec as an Apple launchd plist XML document.
// The output is deterministic: the key order is fixed so writes
// are byte-stable and diffs meaningful.
func renderPlist(spec *schema.LaunchAgentSpec) ([]byte, error) {
	if spec == nil {
		return nil, fmt.Errorf("launchagent: spec is required")
	}

	buf := &bytes.Buffer{}
	buf.WriteString(plistDoctype)
	buf.WriteString(`<plist version="1.0">` + "\n")
	buf.WriteString("<dict>\n")

	writeKey(buf, "Label")
	writeString(buf, spec.Label)

	writeKey(buf, "ProgramArguments")
	writeArray(buf, []string{"/bin/sh", "-c", spec.Command})

	writeKey(buf, "RunAtLoad")
	writeBool(buf, spec.RunAtLoad)

	switch spec.Schedule.Type {
	case schema.ScheduleTypeInterval:
		writeKey(buf, "StartInterval")
		writeInteger(buf, spec.Schedule.IntervalSeconds)
	case schema.ScheduleTypeCalendar:
		writeKey(buf, "StartCalendarInterval")
		writeCalendarInterval(buf, spec.Schedule.Hour, spec.Schedule.Minute)
	default:
		return nil, fmt.Errorf("launchagent: unsupported schedule type %q", string(spec.Schedule.Type))
	}

	buf.WriteString("</dict>\n")
	buf.WriteString("</plist>\n")
	return buf.Bytes(), nil
}

func writeKey(buf *bytes.Buffer, key string) {
	buf.WriteString("  <key>")
	xml.EscapeText(buf, []byte(key))
	buf.WriteString("</key>\n")
}

func writeString(buf *bytes.Buffer, value string) {
	buf.WriteString("  <string>")
	xml.EscapeText(buf, []byte(value))
	buf.WriteString("</string>\n")
}

func writeBool(buf *bytes.Buffer, value bool) {
	if value {
		buf.WriteString("  <true/>\n")
	} else {
		buf.WriteString("  <false/>\n")
	}
}

func writeInteger(buf *bytes.Buffer, value int) {
	buf.WriteString("  <integer>")
	buf.WriteString(strconv.Itoa(value))
	buf.WriteString("</integer>\n")
}

func writeArray(buf *bytes.Buffer, values []string) {
	buf.WriteString("  <array>\n")
	for _, v := range values {
		buf.WriteString("    <string>")
		xml.EscapeText(buf, []byte(v))
		buf.WriteString("</string>\n")
	}
	buf.WriteString("  </array>\n")
}

func writeCalendarInterval(buf *bytes.Buffer, hour, minute int) {
	buf.WriteString("  <dict>\n")
	buf.WriteString("    <key>Hour</key>\n")
	buf.WriteString("    <integer>")
	buf.WriteString(strconv.Itoa(hour))
	buf.WriteString("</integer>\n")
	buf.WriteString("    <key>Minute</key>\n")
	buf.WriteString("    <integer>")
	buf.WriteString(strconv.Itoa(minute))
	buf.WriteString("</integer>\n")
	buf.WriteString("  </dict>\n")
}
