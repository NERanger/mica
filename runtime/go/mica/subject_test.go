package mica

import "testing"

func TestEventSubject(t *testing.T) {
	got := EventSubject("jobs.v1.JobCompleted")
	if got != "event.jobs.v1.JobCompleted" {
		t.Fatalf("got %s", got)
	}
}

func TestRPCSubject(t *testing.T) {
	got := RPCSubjectFromContract("jobs.v1.Worker.Run")
	if got != "rpc.jobs.v1.Worker.Run" {
		t.Fatalf("got %s", got)
	}
}
