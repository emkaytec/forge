package launchagent

import (
	"testing"

	"github.com/emkaytec/forge/pkg/schema"
)

func TestParseLivePlistRoundTrip_Interval(t *testing.T) {
	spec := &schema.LaunchAgentSpec{
		Label:     "com.emkaytec.brew-update",
		Command:   "/opt/homebrew/bin/brew update",
		RunAtLoad: true,
		Schedule: schema.LaunchAgentSchedule{
			Type:            schema.ScheduleTypeInterval,
			IntervalSeconds: 3600,
		},
	}

	data, err := renderPlist(spec)
	if err != nil {
		t.Fatal(err)
	}

	live, err := parseLivePlist(data)
	if err != nil {
		t.Fatal(err)
	}

	got := live.toSpec()
	if got.Label != spec.Label {
		t.Fatalf("Label: want %q, got %q", spec.Label, got.Label)
	}
	if got.Command != spec.Command {
		t.Fatalf("Command: want %q, got %q", spec.Command, got.Command)
	}
	if got.RunAtLoad != spec.RunAtLoad {
		t.Fatalf("RunAtLoad: want %t, got %t", spec.RunAtLoad, got.RunAtLoad)
	}
	if got.Schedule.Type != schema.ScheduleTypeInterval {
		t.Fatalf("Schedule.Type: want interval, got %q", got.Schedule.Type)
	}
	if got.Schedule.IntervalSeconds != 3600 {
		t.Fatalf("Schedule.IntervalSeconds: want 3600, got %d", got.Schedule.IntervalSeconds)
	}
}

func TestParseLivePlistRoundTrip_Calendar(t *testing.T) {
	spec := &schema.LaunchAgentSpec{
		Label:   "com.emkaytec.daily",
		Command: "/usr/bin/true",
		Schedule: schema.LaunchAgentSchedule{
			Type:   schema.ScheduleTypeCalendar,
			Hour:   9,
			Minute: 30,
		},
	}

	data, err := renderPlist(spec)
	if err != nil {
		t.Fatal(err)
	}

	live, err := parseLivePlist(data)
	if err != nil {
		t.Fatal(err)
	}

	got := live.toSpec()
	if got.Schedule.Type != schema.ScheduleTypeCalendar {
		t.Fatalf("Schedule.Type: want calendar, got %q", got.Schedule.Type)
	}
	if got.Schedule.Hour != 9 || got.Schedule.Minute != 30 {
		t.Fatalf("Schedule hour/minute: want 9:30, got %d:%d", got.Schedule.Hour, got.Schedule.Minute)
	}
}

func TestParseLivePlistIgnoresUnknownKeys(t *testing.T) {
	payload := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0">
<dict>
  <key>Label</key><string>com.example.x</string>
  <key>ProgramArguments</key>
  <array><string>/bin/sh</string><string>-c</string><string>/usr/bin/true</string></array>
  <key>StartInterval</key><integer>60</integer>
  <key>SomethingWeDoNotManage</key><string>ignore me</string>
  <key>EnvironmentVariables</key>
  <dict><key>PATH</key><string>/usr/bin</string></dict>
</dict>
</plist>
`)

	live, err := parseLivePlist(payload)
	if err != nil {
		t.Fatal(err)
	}

	if live.Label != "com.example.x" {
		t.Fatalf("Label: want com.example.x, got %q", live.Label)
	}
	if !live.StartIntervalSet || live.StartInterval != 60 {
		t.Fatalf("StartInterval: want set=true value=60, got set=%t value=%d", live.StartIntervalSet, live.StartInterval)
	}
}

func TestExtractCommandFallback(t *testing.T) {
	if got := extractCommand([]string{"/usr/local/bin/tool", "--flag"}); got != "/usr/local/bin/tool --flag" {
		t.Fatalf("unexpected fallback: %q", got)
	}
	if got := extractCommand(nil); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}
