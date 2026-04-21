package workstation

import "strings"

type ProviderKind string

const (
	ProviderAWS     ProviderKind = "aws"
	ProviderGCP     ProviderKind = "gcp"
	ProviderUnknown ProviderKind = "unknown"
)

type Status string

const (
	StatusRunning    Status = "running"
	StatusStopped    Status = "stopped"
	StatusStopping   Status = "stopping"
	StatusPending    Status = "pending"
	StatusTerminated Status = "terminated"
	StatusUnknown    Status = "unknown"
)

// Workstation is the normalized cross-provider view used by the command domain.
type Workstation struct {
	Name              string
	Provider          ProviderKind
	Status            Status
	TailscaleHostname string
	InstanceID        string
	Zone              string
}

func (w Workstation) displayProvider() string {
	switch w.Provider {
	case ProviderAWS:
		return "aws"
	case ProviderGCP:
		return "gcp"
	default:
		return "-"
	}
}

func (w Workstation) displayStatus() string {
	status := strings.TrimSpace(string(w.Status))
	if status == "" {
		return string(StatusUnknown)
	}

	return status
}

func (w Workstation) isRunning() bool {
	return w.Status == StatusRunning
}

func (w Workstation) isStopped() bool {
	switch w.Status {
	case StatusStopped, StatusStopping, StatusTerminated:
		return true
	default:
		return false
	}
}
