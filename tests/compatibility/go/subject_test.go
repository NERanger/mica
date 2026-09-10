package compatibility

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"mica/runtime/go/mica"
)

func TestSubjects(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	path := filepath.Join(filepath.Dir(file), "..", "cases.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	kind := ""
	contract := ""
	expected := ""
	flush := func() {
		if kind == "" {
			return
		}
		var actual string
		if kind == "event" {
			actual = mica.EventSubject(contract)
		} else {
			actual = mica.RPCSubjectFromContract(contract)
		}
		if actual != expected {
			t.Fatalf("%s: got %s expected %s", contract, actual, expected)
		}
		kind = ""
		contract = ""
		expected = ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "[[event]]"):
			flush()
			kind = "event"
		case strings.HasPrefix(line, "[[rpc]]"):
			flush()
			kind = "rpc"
		case strings.HasPrefix(line, "contract"):
			contract = unquote(strings.SplitN(line, "=", 2)[1])
		case strings.HasPrefix(line, "expected_subject"):
			expected = unquote(strings.SplitN(line, "=", 2)[1])
		}
	}
	flush()
}

func unquote(v string) string {
	v = strings.TrimSpace(v)
	return strings.Trim(v, `"`)
}
