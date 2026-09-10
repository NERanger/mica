package mica

type State string

const (
	StateUnspecified State = "UNSPECIFIED"
	StateStarting    State = "STARTING"
	StateRunning     State = "RUNNING"
	StateStopping    State = "STOPPING"
	StateStopped     State = "STOPPED"
	StateFailed      State = "FAILED"
)

type Health struct {
	State    State
	Message  string
	UptimeMS uint64
}
