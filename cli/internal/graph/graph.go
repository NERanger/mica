package graph

import (
	"fmt"
	"strings"

	"mica/cli/internal/manifest"
	"mica/cli/internal/workspace"
)

func ASCII(ws *workspace.Workspace) string {
	type node struct {
		process   string
		component *manifest.Component
	}
	var nodes []node
	for _, process := range ws.App.Processes {
		component, ok := ws.ProcessComponent(process)
		if !ok {
			continue
		}
		nodes = append(nodes, node{process: process.Name, component: component})
	}
	if len(nodes) == 0 {
		return ""
	}
	width := 18
	for _, item := range nodes {
		if len(item.process)+2 > width {
			width = len(item.process) + 2
		}
	}
	languages := map[string]string{}
	for _, item := range nodes {
		languages[item.process] = languageLabel(item.component.Language)
	}
	type edge struct {
		src, dst, label string
	}
	var edges []edge
	for _, item := range nodes {
		for _, contract := range item.component.Publishes {
			for _, other := range nodes {
				if has(other.component.Subscribes, contract) {
					edges = append(edges, edge{item.process, other.process, contract})
				}
			}
		}
		for _, contract := range item.component.Calls {
			for _, other := range nodes {
				if has(other.component.Provides, contract) {
					edges = append(edges, edge{item.process, other.process, "RPC " + contract})
				}
			}
		}
	}
	box := func(name string) []string {
		border := "+" + strings.Repeat("-", width) + "+"
		return []string{
			border,
			"|" + pad(name, width) + "|",
			"|" + pad(languages[name], width) + "|",
			border,
		}
	}
	var lines []string
	boxed := map[string]bool{}
	ensureBox := func(name string) {
		if boxed[name] {
			return
		}
		lines = append(lines, box(name)...)
		boxed[name] = true
	}
	for _, item := range nodes {
		ensureBox(item.process)
		outgoing := []edge{}
		for _, candidate := range edges {
			if candidate.src == item.process {
				outgoing = append(outgoing, candidate)
			}
		}
		for _, candidate := range outgoing {
			indent := strings.Repeat(" ", width/2)
			lines = append(lines, indent+"|")
			lines = append(lines, indent+"| "+candidate.label)
			lines = append(lines, indent+"v")
			if boxed[candidate.dst] {
				lines = append(lines, strings.Repeat(" ", max(0, (width-len(candidate.dst))/2))+candidate.dst)
			} else {
				ensureBox(candidate.dst)
			}
		}
		if len(outgoing) > 0 {
			lines = append(lines, "")
		}
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
}

func DOT(ws *workspace.Workspace) string {
	var out strings.Builder
	fmt.Fprintf(&out, "digraph %q {\n", ws.App.Name)
	out.WriteString("  rankdir=TB;\n")
	for _, process := range ws.App.Processes {
		component, ok := ws.ProcessComponent(process)
		if !ok {
			continue
		}
		fmt.Fprintf(&out, "  %q [label=%q];\n", process.Name, process.Name+"\\n"+component.Language)
	}
	seen := map[string]bool{}
	add := func(src, dst, label string) {
		key := src + "\x00" + dst + "\x00" + label
		if seen[key] {
			return
		}
		seen[key] = true
		fmt.Fprintf(&out, "  %q -> %q [label=%q];\n", src, dst, label)
	}
	for _, process := range ws.App.Processes {
		component, ok := ws.ProcessComponent(process)
		if !ok {
			continue
		}
		for _, contract := range component.Publishes {
			for _, other := range ws.App.Processes {
				otherComponent, ok := ws.ProcessComponent(other)
				if ok && has(otherComponent.Subscribes, contract) {
					add(process.Name, other.Name, contract)
				}
			}
		}
		for _, contract := range component.Calls {
			for _, other := range ws.App.Processes {
				otherComponent, ok := ws.ProcessComponent(other)
				if ok && has(otherComponent.Provides, contract) {
					add(process.Name, other.Name, "RPC "+contract)
				}
			}
		}
	}
	out.WriteString("}\n")
	return out.String()
}

func has(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func languageLabel(language string) string {
	switch language {
	case "python":
		return "Python"
	case "go":
		return "Go"
	case "cpp":
		return "C++"
	}
	return language
}

func pad(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
