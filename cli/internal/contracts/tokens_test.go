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
				Name:    proto.String("jobs/v1/jobs.proto"),
				Package: proto.String("jobs.v1"),
				Options: &descriptorpb.FileOptions{
					GoPackage: proto.String("example.com/app/generated/go/jobs/v1;jobsv1"),
				},
				MessageType: []*descriptorpb.DescriptorProto{
					{Name: proto.String("RunRequest")},
					{Name: proto.String("RunResponse")},
				},
				Service: []*descriptorpb.ServiceDescriptorProto{
					{
						Name: proto.String("Worker"),
						Method: []*descriptorpb.MethodDescriptorProto{
							{
								Name:       proto.String("Run"),
								InputType:  proto.String(".jobs.v1.RunRequest"),
								OutputType: proto.String(".jobs.v1.RunResponse"),
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
	if !strings.Contains(string(python), "class Worker:") {
		t.Fatalf("python tokens missing service class:\n%s", python)
	}
	header, err := os.ReadFile(filepath.Join(generated, "cpp", "mica", "tokens.hpp"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(header), "Worker_Run") {
		t.Fatalf("cpp tokens missing method:\n%s", header)
	}
	goTokens, err := os.ReadFile(filepath.Join(generated, "go", "mica", "tokens", "tokens.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goTokens), "WorkerRun") {
		t.Fatalf("go tokens missing method:\n%s", goTokens)
	}
	if strings.Contains(string(goTokens), "jobsv1.RunRequest.RunRequest") {
		t.Fatalf("go tokens doubled the type name:\n%s", goTokens)
	}
}

func TestLoadImage(t *testing.T) {
	image := testImage(t)
	catalog, err := LoadImage(image)
	if err != nil {
		t.Fatal(err)
	}
	if !catalog.IsRPC("jobs.v1.Worker.Run") {
		t.Fatal("method not catalogued")
	}
	if !catalog.IsEvent("jobs.v1.RunRequest") {
		t.Fatal("message not catalogued")
	}
}
