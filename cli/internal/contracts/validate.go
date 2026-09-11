package contracts

import (
	"fmt"
	"path/filepath"

	"mica/cli/internal/workspace"
)

type Report struct {
	Errors   []string
	Warnings []string
}

func (r *Report) OK() bool {
	return len(r.Errors) == 0
}

func ImagePath(ws *workspace.Workspace) string {
	return filepath.Join(ws.App.GeneratedDir, "image.binpb")
}

func Validate(ws *workspace.Workspace) (*Report, error) {
	catalog, err := LoadImage(ImagePath(ws))
	if err != nil {
		return nil, err
	}
	report := &Report{}
	for _, component := range ws.Components {
		for _, contractID := range append(append([]string{}, component.Publishes...), component.Subscribes...) {
			if !catalog.IsEvent(contractID) {
				report.Errors = append(report.Errors, fmt.Sprintf("component %s: %s is not a known event message", component.Name, contractID))
			}
		}
		for _, contractID := range append(append([]string{}, component.Calls...), component.Provides...) {
			if !catalog.IsRPC(contractID) {
				report.Errors = append(report.Errors, fmt.Sprintf("component %s: %s is not a known RPC method", component.Name, contractID))
			}
		}
	}
	publishers := map[string][]string{}
	subscribers := map[string][]string{}
	callers := map[string][]string{}
	providers := map[string][]string{}
	for _, component := range ws.Components {
		for _, item := range component.Publishes {
			publishers[item] = append(publishers[item], component.Name)
		}
		for _, item := range component.Subscribes {
			subscribers[item] = append(subscribers[item], component.Name)
		}
		for _, item := range component.Calls {
			callers[item] = append(callers[item], component.Name)
		}
		for _, item := range component.Provides {
			providers[item] = append(providers[item], component.Name)
		}
	}
	for contractID, names := range publishers {
		if _, ok := subscribers[contractID]; !ok {
			report.Warnings = append(report.Warnings, fmt.Sprintf("event %s is published by %s but has no subscriber", contractID, join(names)))
		}
	}
	for contractID, names := range subscribers {
		if _, ok := publishers[contractID]; !ok {
			report.Warnings = append(report.Warnings, fmt.Sprintf("event %s is subscribed by %s but has no publisher", contractID, join(names)))
		}
	}
	for contractID, names := range callers {
		if _, ok := providers[contractID]; !ok {
			report.Warnings = append(report.Warnings, fmt.Sprintf("RPC %s is called by %s but has no provider", contractID, join(names)))
		}
	}
	for contractID, names := range providers {
		if _, ok := callers[contractID]; !ok {
			report.Warnings = append(report.Warnings, fmt.Sprintf("RPC %s is provided by %s but is never called", contractID, join(names)))
		}
	}
	return report, nil
}

func join(names []string) string {
	out := ""
	for i, name := range names {
		if i > 0 {
			out += ", "
		}
		out += name
	}
	return out
}
