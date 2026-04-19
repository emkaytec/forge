package schema_test

import (
	"strings"
	"testing"

	"github.com/emkaytec/forge/pkg/schema"
	"gopkg.in/yaml.v3"
)

func TestLaunchAgentRoundTripCalendarSchedule(t *testing.T) {
	t.Parallel()

	manifest, err := schema.DecodeManifest([]byte(`
apiVersion: forge/v1
kind: launch-agent
metadata:
  name: nightly-sync
spec:
  name: nightly-sync
  label: dev.emkaytec.nightly-sync
  command: /usr/local/bin/forge workstation sync
  run_at_load: true
  schedule:
    type: calendar
    hour: 2
    minute: 30
`))
	if err != nil {
		t.Fatalf("DecodeManifest() error = %v", err)
	}

	rendered, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	roundTripped, err := schema.DecodeManifest(rendered)
	if err != nil {
		t.Fatalf("DecodeManifest(roundTrip) error = %v", err)
	}

	spec := roundTripped.Spec.(*schema.LaunchAgentSpec)
	if !spec.RunAtLoad {
		t.Fatal("run_at_load = false, want true")
	}

	if spec.Schedule.Type != schema.ScheduleTypeCalendar {
		t.Fatalf("schedule type = %q, want calendar", spec.Schedule.Type)
	}
}

func TestLaunchAgentRejectsIncompleteSchedule(t *testing.T) {
	t.Parallel()

	_, err := schema.DecodeManifest([]byte(`
apiVersion: forge/v1
kind: launch-agent
metadata:
  name: nightly-sync
spec:
  name: nightly-sync
  label: dev.emkaytec.nightly-sync
  command: /usr/local/bin/forge workstation sync
  schedule:
    type: interval
`))
	if err == nil {
		t.Fatal("DecodeManifest() error = nil, want incomplete schedule error")
	}

	if !strings.Contains(err.Error(), "interval_seconds") {
		t.Fatalf("DecodeManifest() error = %v, want interval_seconds validation", err)
	}
}

func TestLaunchAgentRejectsUnsupportedExtraField(t *testing.T) {
	t.Parallel()

	_, err := schema.DecodeManifest([]byte(`
apiVersion: forge/v1
kind: launch-agent
metadata:
  name: nightly-sync
spec:
  name: nightly-sync
  label: dev.emkaytec.nightly-sync
  command: /usr/local/bin/forge workstation sync
  user: me
  schedule:
    type: interval
    interval_seconds: 600
`))
	if err == nil {
		t.Fatal("DecodeManifest() error = nil, want unsupported extra field")
	}

	if !strings.Contains(err.Error(), "user") {
		t.Fatalf("DecodeManifest() error = %v, want user rejection", err)
	}
}
