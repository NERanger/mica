package launcher

import (
	"testing"

	"mica/cli/internal/spec"
)

func TestVersionSatisfies(t *testing.T) {
	cases := []struct {
		version    string
		constraint string
		want       bool
	}{
		{"3.13.9", ">=3.12", true},
		{"3.11.9", ">=3.12", false},
		{"3.12.0", ">=3.12", true},
		{"2.9.0", ">=2.9.0", true},
		{"2.8.1", ">=2.9.0", false},
		{"5.0.0", "<6", true},
		{"6.0.0", "<6", false},
		{"4.21.12", "==4.21.12", true},
		{"", "", true},
	}
	for _, item := range cases {
		if got := versionSatisfies(item.version, item.constraint); got != item.want {
			t.Errorf("versionSatisfies(%q, %q) = %v, want %v", item.version, item.constraint, got, item.want)
		}
	}
}

func TestSplitConstraint(t *testing.T) {
	name, constraint := splitConstraint("nats-py>=2.9.0")
	if name != "nats-py" || constraint != ">=2.9.0" {
		t.Fatalf("unexpected split: %q %q", name, constraint)
	}
	name, constraint = splitConstraint("protobuf")
	if name != "protobuf" || constraint != "" {
		t.Fatalf("unexpected split: %q %q", name, constraint)
	}
}

func TestSplitConstraints(t *testing.T) {
	name, constraints := splitConstraints("protobuf>=4.21.12,<6")
	if name != "protobuf" || len(constraints) != 2 || constraints[0] != ">=4.21.12" || constraints[1] != "<6" {
		t.Fatalf("unexpected constraints: %q %#v", name, constraints)
	}
	if !versionSatisfies("4.25.0", constraints[0]) || !versionSatisfies("4.25.0", constraints[1]) {
		t.Fatal("expected protobuf constraints to be satisfied")
	}
	if versionSatisfies("6.0.0", constraints[1]) {
		t.Fatal("expected upper bound to reject version 6")
	}
}

func TestTransportFailure(t *testing.T) {
	dep := &spec.Deployment{}
	dep.Transport.Kind = "nats"
	dep.Transport.URL = "nats://127.0.0.1:1"
	if failure := transportFailure(dep); failure == "" {
		t.Fatal("expected unavailable transport failure")
	}
}
