package launchagent

import (
	"strconv"

	"github.com/emkaytec/forge/internal/reconcile"
	"github.com/emkaytec/forge/pkg/schema"
)

// diffSpecs compares desired vs observed and returns the list of
// drift fields. An empty return means the two specs are equivalent
// for reconciliation purposes.
func diffSpecs(desired, observed *schema.LaunchAgentSpec) []reconcile.DriftField {
	var drift []reconcile.DriftField

	if desired.Label != observed.Label {
		drift = append(drift, reconcile.DriftField{
			Path:     "spec.label",
			Desired:  desired.Label,
			Observed: observed.Label,
		})
	}

	if desired.Command != observed.Command {
		drift = append(drift, reconcile.DriftField{
			Path:     "spec.command",
			Desired:  desired.Command,
			Observed: observed.Command,
		})
	}

	if desired.RunAtLoad != observed.RunAtLoad {
		drift = append(drift, reconcile.DriftField{
			Path:     "spec.run_at_load",
			Desired:  strconv.FormatBool(desired.RunAtLoad),
			Observed: strconv.FormatBool(observed.RunAtLoad),
		})
	}

	drift = append(drift, diffSchedule(desired.Schedule, observed.Schedule)...)

	return drift
}

func diffSchedule(desired, observed schema.LaunchAgentSchedule) []reconcile.DriftField {
	if desired.Type != observed.Type {
		return []reconcile.DriftField{{
			Path:     "spec.schedule.type",
			Desired:  string(desired.Type),
			Observed: string(observed.Type),
		}}
	}

	var drift []reconcile.DriftField
	switch desired.Type {
	case schema.ScheduleTypeInterval:
		if desired.IntervalSeconds != observed.IntervalSeconds {
			drift = append(drift, reconcile.DriftField{
				Path:     "spec.schedule.interval_seconds",
				Desired:  strconv.Itoa(desired.IntervalSeconds),
				Observed: strconv.Itoa(observed.IntervalSeconds),
			})
		}
	case schema.ScheduleTypeCalendar:
		if desired.Hour != observed.Hour {
			drift = append(drift, reconcile.DriftField{
				Path:     "spec.schedule.hour",
				Desired:  strconv.Itoa(desired.Hour),
				Observed: strconv.Itoa(observed.Hour),
			})
		}
		if desired.Minute != observed.Minute {
			drift = append(drift, reconcile.DriftField{
				Path:     "spec.schedule.minute",
				Desired:  strconv.Itoa(desired.Minute),
				Observed: strconv.Itoa(observed.Minute),
			})
		}
	}

	return drift
}
