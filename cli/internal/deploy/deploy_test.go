package deploy

import (
	"testing"

	"mica/cli/internal/manifest"
)

func TestNormalizePlatform(t *testing.T) {
	if got := normalizeOS("Linux"); got != "linux" {
		t.Fatalf("normalizeOS = %q", got)
	}
	if got := normalizeArch("x86_64"); got != "amd64" {
		t.Fatalf("normalizeArch = %q", got)
	}
	if got := normalizeArch("aarch64"); got != "arm64" {
		t.Fatalf("normalizeArch = %q", got)
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("/tmp/mica deploy/app.run"); got != "'/tmp/mica deploy/app.run'" {
		t.Fatalf("shellQuote = %q", got)
	}
	if got := shellQuote("it's"); got != `'it'"'"'s'` {
		t.Fatalf("shellQuote = %q", got)
	}
}

func TestPythonRuntimeRequirement(t *testing.T) {
	component := &manifest.Component{
		Name:     "recorder",
		Artifact: manifest.ArtifactSpec{Kind: manifest.ArtifactPythonSource},
		Requirements: manifest.Requirements{
			Python: &manifest.PythonRequirements{Packages: []string{"protobuf>=4.21.12,<6"}},
		},
	}
	requirement, ok := requirementFor(component, false)
	if !ok || len(requirement.PythonPackages) != 2 || requirement.PythonPackages[1] != "mica" {
		t.Fatalf("expected mica host requirement: %+v", requirement)
	}
	requirement, ok = requirementFor(component, true)
	if !ok || len(requirement.PythonPackages) != 1 {
		t.Fatalf("did not expect bundled runtime requirement: %+v", requirement)
	}
}
