package workstation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

type provider interface {
	Kind() ProviderKind
	List(ctx context.Context) ([]Workstation, error)
	Start(ctx context.Context, workstation Workstation) error
	Stop(ctx context.Context, workstation Workstation) error
}

type awsProvider struct {
	runner commandRunner
}

func newAWSProvider(runner commandRunner) provider {
	return &awsProvider{runner: runner}
}

func (p *awsProvider) Kind() ProviderKind { return ProviderAWS }

func (p *awsProvider) List(ctx context.Context) ([]Workstation, error) {
	if _, err := p.runner.LookPath("aws"); err != nil {
		return nil, &binaryUnavailableError{provider: p.Kind(), binary: "aws", err: err}
	}

	output, err := p.runner.Output(
		ctx,
		"aws",
		"ec2",
		"describe-instances",
		"--filters",
		"Name=tag:forge-managed,Values=true",
		"Name=tag:forge-role,Values=workstation",
		"Name=instance-state-name,Values=pending,running,stopping,stopped",
		"--output",
		"json",
	)
	if err != nil {
		return nil, err
	}

	var response struct {
		Reservations []struct {
			Instances []struct {
				InstanceID string `json:"InstanceId"`
				State      struct {
					Name string `json:"Name"`
				} `json:"State"`
				Placement struct {
					AvailabilityZone string `json:"AvailabilityZone"`
				} `json:"Placement"`
				Tags []struct {
					Key   string `json:"Key"`
					Value string `json:"Value"`
				} `json:"Tags"`
			} `json:"Instances"`
		} `json:"Reservations"`
	}

	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("decode aws workstation list: %w", err)
	}

	var workstations []Workstation
	for _, reservation := range response.Reservations {
		for _, instance := range reservation.Instances {
			tags := make(map[string]string, len(instance.Tags))
			for _, tag := range instance.Tags {
				tags[strings.TrimSpace(tag.Key)] = strings.TrimSpace(tag.Value)
			}

			name := firstNonEmpty(tags["forge-name"], tags["Name"], instance.InstanceID)
			workstations = append(workstations, Workstation{
				Name:              name,
				Provider:          ProviderAWS,
				Status:            normalizeStatus(instance.State.Name),
				TailscaleHostname: tailscaleHostname(tags),
				InstanceID:        instance.InstanceID,
				Zone:              instance.Placement.AvailabilityZone,
			})
		}
	}

	return sortWorkstations(workstations), nil
}

func (p *awsProvider) Start(ctx context.Context, workstation Workstation) error {
	if strings.TrimSpace(workstation.InstanceID) == "" {
		return fmt.Errorf("aws workstation %s is missing an instance ID", workstation.Name)
	}

	if _, err := p.runner.LookPath("aws"); err != nil {
		return &binaryUnavailableError{provider: p.Kind(), binary: "aws", err: err}
	}

	_, err := p.runner.Output(ctx, "aws", "ec2", "start-instances", "--instance-ids", workstation.InstanceID, "--output", "json")
	return err
}

func (p *awsProvider) Stop(ctx context.Context, workstation Workstation) error {
	if strings.TrimSpace(workstation.InstanceID) == "" {
		return fmt.Errorf("aws workstation %s is missing an instance ID", workstation.Name)
	}

	if _, err := p.runner.LookPath("aws"); err != nil {
		return &binaryUnavailableError{provider: p.Kind(), binary: "aws", err: err}
	}

	_, err := p.runner.Output(ctx, "aws", "ec2", "stop-instances", "--instance-ids", workstation.InstanceID, "--output", "json")
	return err
}

type gcpProvider struct {
	runner commandRunner
}

func newGCPProvider(runner commandRunner) provider {
	return &gcpProvider{runner: runner}
}

func (p *gcpProvider) Kind() ProviderKind { return ProviderGCP }

func (p *gcpProvider) List(ctx context.Context) ([]Workstation, error) {
	if _, err := p.runner.LookPath("gcloud"); err != nil {
		return nil, &binaryUnavailableError{provider: p.Kind(), binary: "gcloud", err: err}
	}

	output, err := p.runner.Output(
		ctx,
		"gcloud",
		"compute",
		"instances",
		"list",
		"--format=json",
		"--filter=labels.forge-managed=true AND labels.forge-role=workstation",
	)
	if err != nil {
		return nil, err
	}

	var instances []struct {
		Name   string            `json:"name"`
		Status string            `json:"status"`
		Zone   string            `json:"zone"`
		Labels map[string]string `json:"labels"`
	}

	if err := json.Unmarshal(output, &instances); err != nil {
		return nil, fmt.Errorf("decode gcp workstation list: %w", err)
	}

	workstations := make([]Workstation, 0, len(instances))
	for _, instance := range instances {
		name := firstNonEmpty(instance.Labels["forge-name"], instance.Name)
		workstations = append(workstations, Workstation{
			Name:              name,
			Provider:          ProviderGCP,
			Status:            normalizeStatus(instance.Status),
			TailscaleHostname: tailscaleHostname(instance.Labels),
			InstanceID:        instance.Name,
			Zone:              zoneName(instance.Zone),
		})
	}

	return sortWorkstations(workstations), nil
}

func (p *gcpProvider) Start(ctx context.Context, workstation Workstation) error {
	if strings.TrimSpace(workstation.InstanceID) == "" {
		return fmt.Errorf("gcp workstation %s is missing an instance name", workstation.Name)
	}
	if strings.TrimSpace(workstation.Zone) == "" {
		return fmt.Errorf("gcp workstation %s is missing a zone", workstation.Name)
	}

	if _, err := p.runner.LookPath("gcloud"); err != nil {
		return &binaryUnavailableError{provider: p.Kind(), binary: "gcloud", err: err}
	}

	_, err := p.runner.Output(ctx, "gcloud", "compute", "instances", "start", workstation.InstanceID, "--zone", workstation.Zone, "--format=json")
	return err
}

func (p *gcpProvider) Stop(ctx context.Context, workstation Workstation) error {
	if strings.TrimSpace(workstation.InstanceID) == "" {
		return fmt.Errorf("gcp workstation %s is missing an instance name", workstation.Name)
	}
	if strings.TrimSpace(workstation.Zone) == "" {
		return fmt.Errorf("gcp workstation %s is missing a zone", workstation.Name)
	}

	if _, err := p.runner.LookPath("gcloud"); err != nil {
		return &binaryUnavailableError{provider: p.Kind(), binary: "gcloud", err: err}
	}

	_, err := p.runner.Output(ctx, "gcloud", "compute", "instances", "stop", workstation.InstanceID, "--zone", workstation.Zone, "--format=json")
	return err
}

func normalizeStatus(value string) Status {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "running":
		return StatusRunning
	case "stopped", "terminated":
		return StatusStopped
	case "stopping":
		return StatusStopping
	case "pending", "provisioning", "starting":
		return StatusPending
	default:
		return StatusUnknown
	}
}

func tailscaleHostname(values map[string]string) string {
	return firstNonEmpty(
		values["tailscale-hostname"],
		values["tailscale_hostname"],
		values["forge-tailscale-hostname"],
	)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func zoneName(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed == "" {
		return ""
	} else if parsed, err := url.Parse(trimmed); err == nil && parsed.Path != "" {
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		return parts[len(parts)-1]
	}

	return value
}

func sortWorkstations(workstations []Workstation) []Workstation {
	sort.Slice(workstations, func(i, j int) bool {
		if workstations[i].Name == workstations[j].Name {
			return workstations[i].Provider < workstations[j].Provider
		}
		return workstations[i].Name < workstations[j].Name
	})
	return workstations
}
