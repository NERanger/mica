package mica

import "strings"

func NormalizeFullName(name string) string {
	return strings.TrimPrefix(name, ".")
}

func EventSubject(messageFullName string) string {
	return "event." + NormalizeFullName(messageFullName)
}

func RPCSubject(serviceFullName, method string) string {
	return "rpc." + NormalizeFullName(serviceFullName) + "." + method
}

func SplitRPCContract(contractID string) (service, method string) {
	name := NormalizeFullName(contractID)
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return name, ""
	}
	return name[:idx], name[idx+1:]
}

func RPCSubjectFromContract(contractID string) string {
	service, method := SplitRPCContract(contractID)
	return RPCSubject(service, method)
}
