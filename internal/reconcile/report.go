package reconcile

import (
	"fmt"
	"io"
	"strings"

	"github.com/emkaytec/forge/internal/ui"
)

// RenderPlan writes a human-readable summary of plan to w, using the
// shared ui styling primitives. Layout:
//
//	Plan (<target>)
//	  <kind> <name>    <action>  [<note>]
//	    <path>: <desired> -> <observed>    (per drift field)
//
//	Skipped
//	  <kind> <name>    <reason>
//
//	Errors
//	  <source>: <message>
func RenderPlan(w io.Writer, plan *Plan) {
	fmt.Fprintln(w, ui.RenderHeading(w, fmt.Sprintf("Plan (%s)", plan.Target)))

	if len(plan.Changes) == 0 {
		fmt.Fprintln(w, "  "+ui.RenderMuted(w, "no changes planned"))
	} else {
		for _, change := range plan.Changes {
			writeChange(w, change)
		}
	}

	if len(plan.Skipped) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.RenderHeading(w, "Skipped"))
		for _, skipped := range plan.Skipped {
			writeSkipped(w, skipped)
		}
	}

	if len(plan.LoadErrors) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.RenderHeading(w, "Errors"))
		for _, e := range plan.LoadErrors {
			ui.Error(w, e.Error())
		}
	}
}

// RenderApplyResult writes the outcome of Executor.Apply to w.
func RenderApplyResult(w io.Writer, result *ApplyResult) {
	heading := fmt.Sprintf("Applied (%s)", result.Target)
	if result.DryRun {
		heading = fmt.Sprintf("Dry run (%s)", result.Target)
	}
	fmt.Fprintln(w, ui.RenderHeading(w, heading))

	if len(result.Applied) == 0 {
		fmt.Fprintln(w, "  "+ui.RenderMuted(w, "no changes applied"))
	} else {
		for _, change := range result.Applied {
			writeChange(w, change)
		}
	}

	if len(result.Skipped) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.RenderHeading(w, "Skipped"))
		for _, skipped := range result.Skipped {
			writeSkipped(w, skipped)
		}
	}

	if len(result.Failed) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, ui.RenderHeading(w, "Failed"))
		for _, f := range result.Failed {
			ui.Error(w, fmt.Sprintf("%s %s: %s", f.Change.Kind(), f.Change.Name(), f.Err.Error()))
		}
	}
}

func writeChange(w io.Writer, change ResourceChange) {
	header := fmt.Sprintf("%s %s", change.Kind(), change.Name())
	action := fmt.Sprintf("[%s]", change.Action)

	switch change.Action {
	case ActionNoOp:
		ui.Success(w, strings.TrimSpace(fmt.Sprintf("%s %s %s", header, action, change.Note)))
	case ActionDelete:
		ui.Error(w, strings.TrimSpace(fmt.Sprintf("%s %s %s", header, action, change.Note)))
	default:
		ui.Warn(w, strings.TrimSpace(fmt.Sprintf("%s %s %s", header, action, change.Note)))
	}

	for _, drift := range change.Drift {
		fmt.Fprintf(w, "    %s: %s -> %s\n", drift.Path, formatValue(drift.Desired), formatValue(drift.Observed))
	}
}

func writeSkipped(w io.Writer, skipped ResourceChange) {
	ui.Warn(w, fmt.Sprintf("%s %s: %s", skipped.Kind(), skipped.Name(), skipped.SkipReason))
}

func formatValue(v string) string {
	if v == "" {
		return "(unset)"
	}
	return v
}
