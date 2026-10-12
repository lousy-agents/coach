package domain

import (
	"sort"
)

// Workspace is one language workspace discovered in a snapshot.
type Workspace struct {
	ID       string   `json:"id"`
	Language string   `json:"language"`
	Root     string   `json:"root"`
	Projects []string `json:"projects,omitempty"`
}

// Module is one module discovered in a snapshot.
type Module struct {
	ID       string   `json:"id"`
	Path     string   `json:"path"`
	Language string   `json:"language"`
	Files    []string `json:"files,omitempty"`
}

// Package is one package discovered in a snapshot.
type Package struct {
	ID       string   `json:"id"`
	Path     string   `json:"path"`
	Language string   `json:"language"`
	Files    []string `json:"files,omitempty"`
}

func canonicalWorkspaces(in []Workspace) []Workspace {
	if len(in) == 0 {
		return in
	}
	out := append([]Workspace(nil), in...)
	for i := range out {
		out[i].Projects = sortedStrings(out[i].Projects)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		if out[i].Root != out[j].Root {
			return out[i].Root < out[j].Root
		}
		return out[i].Language < out[j].Language
	})
	return out
}

func canonicalModules(in []Module) []Module {
	if len(in) == 0 {
		return in
	}
	out := append([]Module(nil), in...)
	for i := range out {
		out[i].Files = sortedStrings(out[i].Files)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func canonicalPackages(in []Package) []Package {
	if len(in) == 0 {
		return in
	}
	out := append([]Package(nil), in...)
	for i := range out {
		out[i].Files = sortedStrings(out[i].Files)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func sortedStrings(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}
