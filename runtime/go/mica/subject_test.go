package mica

import "testing"

func TestEventSubject(t *testing.T) {
	got := EventSubject("camera.v1.PoseChanged")
	if got != "event.camera.v1.PoseChanged" {
		t.Fatalf("got %s", got)
	}
}

func TestRPCSubject(t *testing.T) {
	got := RPCSubjectFromContract("camera.v1.CameraControl.SetPose")
	if got != "rpc.camera.v1.CameraControl.SetPose" {
		t.Fatalf("got %s", got)
	}
}
