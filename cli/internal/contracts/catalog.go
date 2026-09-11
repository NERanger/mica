package contracts

import (
	"fmt"
	"os"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Catalog struct {
	Messages map[string]bool
	Services map[string]bool
	Methods  map[string]bool
}

func LoadImage(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("missing descriptor image %s; run mica generate", path)
	}
	fds := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(data, fds); err != nil {
		return nil, fmt.Errorf("invalid descriptor image %s: %w", path, err)
	}
	catalog := &Catalog{
		Messages: map[string]bool{},
		Services: map[string]bool{},
		Methods:  map[string]bool{},
	}
	for _, file := range fds.GetFile() {
		pkg := file.GetPackage()
		for _, message := range file.GetMessageType() {
			catalog.Messages[qualify(pkg, message.GetName())] = true
		}
		for _, service := range file.GetService() {
			name := qualify(pkg, service.GetName())
			catalog.Services[name] = true
			for _, method := range service.GetMethod() {
				catalog.Methods[name+"."+method.GetName()] = true
			}
		}
	}
	return catalog, nil
}

func (c *Catalog) IsEvent(contractID string) bool {
	return c.Messages[contractID]
}

func (c *Catalog) IsRPC(contractID string) bool {
	return c.Methods[contractID]
}

func qualify(pkg, name string) string {
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}
