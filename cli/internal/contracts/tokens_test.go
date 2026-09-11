package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func testImage(t *testing.T) string {
	t.Helper()
	fds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("camera/v1/camera.proto"),
				Package: proto.String("camera.v1"),
				Options: &descriptorpb.FileOptions{
					GoPackage: proto.String("example.com/app/generated/go/camera/v1;camerav1"),
				},
				MessageType: []*descriptorpb.DescriptorProto{
					{Name: proto.String("SetPoseRequest")},
					{Name: proto.String("SetPoseResponse")},
				},
				Service: []*descriptorpb.ServiceDescriptorProto{
					{
						Name: proto.String("CameraControl"),
						Method: []*descriptorpb.MethodDescriptorProto{
							{
								Name:       proto.String("SetPose"),
								InputType:  proto.String(".camera.v1.SetPoseRequest"),
								OutputType: proto.String(".camera.v1.SetPoseResponse"),
							},
						},
					},
				},
			},
		},
	}
	data, err := proto.Marshal(fds)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "image.binpb")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWriteTokens(t *testing.T) {
	image := testImage(t)
	generated := t.TempDir()
	if err := WriteTokens(image, generated, "mica/runtime/go/mica"); err != nil {
		t.Fatal(err)
	}
	python, err := os.ReadFile(filepath.Join(generated, "python", "mica_tokens.py"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(python), "class CameraControl:") {
		t.Fatalf("python tokens missing service class:\n%s", python)
	}
	header, err := os.ReadFile(filepath.Join(generated, "cpp", "mica", "tokens.hpp"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(header), "CameraControl_SetPose") {
		t.Fatalf("cpp tokens missing method:\n%s", header)
	}
	goTokens, err := os.ReadFile(filepath.Join(generated, "go", "mica", "tokens", "tokens.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goTokens), "CameraControlSetPose") {
		t.Fatalf("go tokens missing method:\n%s", goTokens)
	}
	if strings.Contains(string(goTokens), "camerav1.SetPoseRequest.SetPoseRequest") {
		t.Fatalf("go tokens doubled the type name:\n%s", goTokens)
	}
}

func TestLoadImage(t *testing.T) {
	image := testImage(t)
	catalog, err := LoadImage(image)
	if err != nil {
		t.Fatal(err)
	}
	if !catalog.IsRPC("camera.v1.CameraControl.SetPose") {
		t.Fatal("method not catalogued")
	}
	if !catalog.IsEvent("camera.v1.SetPoseRequest") {
		t.Fatal("message not catalogued")
	}
}
