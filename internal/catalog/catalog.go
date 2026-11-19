// Package catalog loads template manifests and answers catalog queries.
package catalog

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Param is one user-supplied input of a template.
type Param struct {
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description"`
	Default     string   `yaml:"default" json:"default"`
	Required    bool     `yaml:"required" json:"required"`
	Targets     []string `yaml:"targets" json:"targets,omitempty"` // empty = all targets
}

// AppliesTo reports whether the parameter is used by the given target.
func (p Param) AppliesTo(target string) bool {
	if len(p.Targets) == 0 {
		return true
	}
	for _, t := range p.Targets {
		if t == target {
			return true
		}
	}
	return false
}

// Target describes what a template produces for one deployment target.
type Target struct {
	Summary   string   `yaml:"summary" json:"summary"`
	Layers    []string `yaml:"layers" json:"layers,omitempty"` // shared directories applied before the target's own
	Resources []string `yaml:"resources" json:"resources"`
	Requires  []string `yaml:"requires" json:"requires"` // CLI tools needed on PATH
}

// Template is one catalog entry.
type Template struct {
	ID          string            `yaml:"id" json:"id"`
	Name        string            `yaml:"name" json:"name"`
	Category    string            `yaml:"category" json:"category"`
	Description string            `yaml:"description" json:"description"`
	Version     string            `yaml:"version" json:"version"`
	Tags        []string          `yaml:"tags" json:"tags"`
	Parameters  []Param           `yaml:"parameters" json:"parameters"`
	Targets     map[string]Target `yaml:"targets" json:"targets"`

	FS fs.FS `yaml:"-" json:"-"` // root of this template's directory
}

// Layers returns the directories to overlay, in order, for a target:
// base, then any shared layers the target lists, then targets/<target>.
func (t *Template) Layers(target string) ([]string, error) {
	tg, ok := t.Targets[target]
	if !ok {
		return nil, fmt.Errorf("template %q has no target %q", t.ID, target)
	}
	out := []string{"base"}
	for _, l := range tg.Layers {
		if _, err := fs.Stat(t.FS, l); err != nil {
			return nil, fmt.Errorf("template %q target %q: layer %q not found", t.ID, target, l)
		}
		out = append(out, l)
	}
	return append(out, "targets/"+target), nil
}

// Catalog is the set of loaded templates.
type Catalog struct {
	byID map[string]*Template
}

// Load reads every top-level directory of fsys that contains a template.yaml.
func Load(fsys fs.FS) (*Catalog, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	c := &Catalog{byID: map[string]*Template{}}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		raw, err := fs.ReadFile(fsys, e.Name()+"/template.yaml")
		if err != nil {
			continue // not a template directory
		}
		var t Template
		if err := yaml.Unmarshal(raw, &t); err != nil {
			return nil, fmt.Errorf("%s/template.yaml: %w", e.Name(), err)
		}
		if t.ID != e.Name() {
			return nil, fmt.Errorf("%s/template.yaml: id %q must match the directory name", e.Name(), t.ID)
		}
		if len(t.Targets) == 0 {
			return nil, fmt.Errorf("template %q declares no targets", t.ID)
		}
		if t.FS, err = fs.Sub(fsys, e.Name()); err != nil {
			return nil, err
		}
		for name := range t.Targets {
			if _, err := t.Layers(name); err != nil {
				return nil, err
			}
		}
		c.byID[t.ID] = &t
	}
	if len(c.byID) == 0 {
		return nil, fmt.Errorf("no templates found")
	}
	return c, nil
}

// Get returns a template by id.
func (c *Catalog) Get(id string) (*Template, bool) {
	t, ok := c.byID[id]
	return t, ok
}

// Search returns templates matching a free-text query and optional target,
// best match first. An empty query returns everything sorted by category.
func (c *Catalog) Search(query, target string) []*Template {
	words := strings.Fields(strings.ToLower(query))
	type hit struct {
		t     *Template
		score int
	}
	var hits []hit
	for _, t := range c.byID {
		if target != "" {
			if _, ok := t.Targets[target]; !ok {
				continue
			}
		}
		score := 0
		for _, w := range words {
			score += 5 * strings.Count(strings.ToLower(t.Name+" "+t.ID), w)
			score += 3 * strings.Count(strings.ToLower(strings.Join(t.Tags, " ")+" "+t.Category), w)
			score += strings.Count(strings.ToLower(t.Description), w)
			for _, tg := range t.Targets {
				score += 2 * strings.Count(strings.ToLower(tg.Summary+" "+strings.Join(tg.Resources, " ")), w)
			}
		}
		if len(words) > 0 && score == 0 {
			continue
		}
		hits = append(hits, hit{t, score})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		if hits[i].t.Category != hits[j].t.Category {
			return hits[i].t.Category < hits[j].t.Category
		}
		return hits[i].t.ID < hits[j].t.ID
	})
	out := make([]*Template, len(hits))
	for i, h := range hits {
		out[i] = h.t
	}
	return out
}
