package launcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"mica/cli/internal/spec"
)

var surfaceKinds = []string{"publishes", "subscribes", "calls", "provides"}

var surfaceObservedLabels = map[string]string{
	"publishes":  "published event",
	"subscribes": "subscribed event",
	"calls":      "called RPC",
	"provides":   "served RPC",
}

// registrationKinds are recorded deterministically at app start, so a declared
// contract missing from the observed surface is drift rather than an
// unexercised code path.
var registrationKinds = []string{"subscribes", "provides"}

type observedSurface struct {
	Publishes  []string `json:"publishes"`
	Subscribes []string `json:"subscribes"`
	Calls      []string `json:"calls"`
	Provides   []string `json:"provides"`
}

func surfacePath(root, processName string) string {
	return filepath.Join(root, "surface", processName+".json")
}

func reportSurface(root string, process spec.Process) {
	warnings := surfaceDrift(root, process)
	if len(warnings) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "mica: %s: contract surface differs from component.toml:\n", process.Name)
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "  - %s\n", warning)
	}
}

func surfaceDrift(root string, process spec.Process) []string {
	body, err := os.ReadFile(surfacePath(root, process.Name))
	if err != nil {
		return nil
	}
	var observed observedSurface
	if err := json.Unmarshal(body, &observed); err != nil {
		return nil
	}
	declared := map[string][]string{
		"publishes":  process.Publishes,
		"subscribes": process.Subscribes,
		"calls":      process.Calls,
		"provides":   process.Provides,
	}
	seen := map[string][]string{
		"publishes":  observed.Publishes,
		"subscribes": observed.Subscribes,
		"calls":      observed.Calls,
		"provides":   observed.Provides,
	}
	var warnings []string
	for _, kind := range surfaceKinds {
		for _, id := range seen[kind] {
			if !containsString(declared[kind], id) {
				warnings = append(warnings, fmt.Sprintf("%s %s is not declared in component.toml", surfaceObservedLabels[kind], id))
			}
		}
	}
	for _, kind := range registrationKinds {
		for _, id := range declared[kind] {
			if !containsString(seen[kind], id) {
				warnings = append(warnings, fmt.Sprintf("component.toml declares %s %s but no handler was registered", kind, id))
			}
		}
	}
	return warnings
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
