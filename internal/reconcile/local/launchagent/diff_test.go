package launchagent

import (
	"testing"

	"github.com/emkaytec/forge/pkg/schema"
)

func TestDiffSpecs_NoDrift(t *testing.T) {
	a := sampleIntervalSpec()
	b := sampleIntervalSpec()

	if got := diffSpecs(a, b); len(got) != 0 {
		t.Fatalf("want no drift, got %+v", got)
	}
}

func TestDiffSpecs_CommandDrift(t *testing.T) {
	a := sampleIntervalSpec()
	b := sampleIntervalSpec()
	b.Command = "/usr/bin/false"

	drift := diffSpecs(a, b)
	if len(drift) != 1 {
		t.Fatalf("want 1 drift entry, got %d (%+v)", len(drift), drift)
	}
	if drift[0].Path != "spec.command" {
		t.Fatalf("want spec.command drift, got %q", drift[0].Path)
	}
	if drift[0].Desired != a.Command || drift[0].Observed != b.Command {
		t.Fatalf("desired/observed mismatch: %+v", drift[0])
	}
}

func TestDiffSpecs_ScheduleTypeChange(t *testing.T) {
	a := sampleIntervalSpec()
	b := sampleIntervalSpec()
	b.Schedule = schema.LaunchAgentSchedule{Type: schema.ScheduleTypeCalendar, Hour: 9, Minute: 0}

	drift := diffSpecs(a, b)
	if len(drift) != 1 || drift[0].Path != "spec.schedule.type" {
		t.Fatalf("want spec.schedule.type drift, got %+v", drift)
	}
}

func TestDiffSpecs_CalendarMinute(t *testing.T) {
	a := &schema.LaunchAgentSpec{
		Label:   "x",
		Command: "/usr/bin/true",
		Schedule: schema.LaunchAgentSchedule{Type: schema.ScheduleTypeCalendar, Hour: 9, Minute: 0},
	}
	b := &schema.LaunchAgentSpec{
		Label:   "x",
		Command: "/usr/bin/true",
		Schedule: schema.LaunchAgentSchedule{Type: schema.ScheduleTypeCalendar, Hour: 9, Minute: 30},
	}

	drift := diffSpecs(a, b)
	if len(drift) != 1 || drift[0].Path != "spec.schedule.minute" {
		t.Fatalf("want spec.schedule.minute drift, got %+v", drift)
	}
}

func sampleIntervalSpec() *schema.LaunchAgentSpec {
	return &schema.LaunchAgentSpec{
		Label:     "com.emkaytec.x",
		Command:   "/usr/bin/true",
		RunAtLoad: true,
		Schedule: schema.LaunchAgentSchedule{
			Type:            schema.ScheduleTypeInterval,
			IntervalSeconds: 60,
		},
	}
}
