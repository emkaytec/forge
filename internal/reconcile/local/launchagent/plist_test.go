package launchagent

import (
	"strings"
	"testing"

	"github.com/emkaytec/forge/pkg/schema"
)

func TestRenderPlistInterval(t *testing.T) {
	spec := &schema.LaunchAgentSpec{
		Label:     "com.emkaytec.brew-update",
		Command:   "/opt/homebrew/bin/brew update",
		RunAtLoad: true,
		Schedule: schema.LaunchAgentSchedule{
			Type:            schema.ScheduleTypeInterval,
			IntervalSeconds: 3600,
		},
	}

	got, err := renderPlist(spec)
	if err != nil {
		t.Fatal(err)
	}

	assertContains(t, string(got), `<key>Label</key>`, `<string>com.emkaytec.brew-update</string>`)
	assertContains(t, string(got), `<key>ProgramArguments</key>`, `<string>/bin/sh</string>`, `<string>-c</string>`, `<string>/opt/homebrew/bin/brew update</string>`)
	assertContains(t, string(got), `<key>RunAtLoad</key>`, `<true/>`)
	assertContains(t, string(got), `<key>StartInterval</key>`, `<integer>3600</integer>`)
}

func TestRenderPlistCalendar(t *testing.T) {
	spec := &schema.LaunchAgentSpec{
		Label:   "com.emkaytec.daily",
		Command: "/usr/bin/true",
		Schedule: schema.LaunchAgentSchedule{
			Type:   schema.ScheduleTypeCalendar,
			Hour:   9,
			Minute: 30,
		},
	}

	got, err := renderPlist(spec)
	if err != nil {
		t.Fatal(err)
	}

	assertContains(t, string(got), `<key>StartCalendarInterval</key>`)
	assertContains(t, string(got), `<key>Hour</key>`, `<integer>9</integer>`)
	assertContains(t, string(got), `<key>Minute</key>`, `<integer>30</integer>`)
	assertContains(t, string(got), `<false/>`) // RunAtLoad default
}

func TestRenderPlistEscapesCommand(t *testing.T) {
	spec := &schema.LaunchAgentSpec{
		Label:   "com.emkaytec.xml",
		Command: `echo "hello" && echo <world>`,
		Schedule: schema.LaunchAgentSchedule{
			Type:            schema.ScheduleTypeInterval,
			IntervalSeconds: 60,
		},
	}

	got, err := renderPlist(spec)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "<world>") {
		t.Fatalf("unescaped XML in plist output: %s", got)
	}
	assertContains(t, string(got), "&lt;world&gt;", "&#34;hello&#34;")
}

func TestRenderPlistRejectsUnknownScheduleType(t *testing.T) {
	spec := &schema.LaunchAgentSpec{
		Label:    "com.emkaytec.x",
		Command:  "/usr/bin/true",
		Schedule: schema.LaunchAgentSchedule{Type: "monthly"},
	}

	if _, err := renderPlist(spec); err == nil {
		t.Fatal("want error for unknown schedule type, got nil")
	}
}

func assertContains(t *testing.T, haystack string, needles ...string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(haystack, needle) {
			t.Fatalf("expected %q in output:\n%s", needle, haystack)
		}
	}
}
