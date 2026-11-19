package engine

import (
	"bytes"
	"strings"
	"testing"

	"github.com/udaykishore-resu/kickhub/internal/catalog"
	"github.com/udaykishore-resu/kickhub/templates"
)

func load(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalog.Load(templates.FS)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// Every built-in template must render cleanly for every target it declares.
func TestEveryTemplateRendersForEveryTarget(t *testing.T) {
	c := load(t)
	for _, tpl := range c.Search("", "") {
		for target := range tpl.Targets {
			t.Run(tpl.ID+"/"+target, func(t *testing.T) {
				res, err := Render(c, Request{Template: tpl.ID, Target: target, Name: "demo-svc",
					Params: map[string]string{"gcp_project": "demo-project"}})
				if err != nil {
					t.Fatal(err)
				}
				for _, want := range []string{"README.md", "Makefile", ".kickhub.yaml", ".gitignore"} {
					if _, ok := res.Files[want]; !ok {
						t.Errorf("missing %s", want)
					}
				}
				for p, body := range res.Files {
					if strings.HasSuffix(p, ".tmpl") {
						t.Errorf("%s: .tmpl suffix was not stripped", p)
					}
					if bytes.Contains(body, []byte("[[")) || bytes.Contains(body, []byte("<no value>")) {
						t.Errorf("%s: unrendered template syntax", p)
					}
				}
				_, hasInfra := res.Files["infra/main.tf"]
				_, hasCompose := res.Files["docker-compose.yml"]
				if (target == "local") != hasCompose || (target != "local") != hasInfra {
					t.Errorf("target %s: infra=%v compose=%v", target, hasInfra, hasCompose)
				}
			})
		}
	}
}

func TestRenderRejectsBadInput(t *testing.T) {
	c := load(t)
	cases := map[string]Request{
		"unknown template":  {Template: "nope", Target: "aws", Name: "demo-svc"},
		"unknown target":    {Template: "http-api", Target: "azure", Name: "demo-svc"},
		"bad name":          {Template: "http-api", Target: "aws", Name: "Demo Svc"},
		"path traversal":    {Template: "http-api", Target: "aws", Name: "../../etc"},
		"missing required":  {Template: "http-api", Target: "gcp", Name: "demo-svc"},
		"unknown parameter": {Template: "http-api", Target: "aws", Name: "demo-svc", Params: map[string]string{"nope": "x"}},
		"quote injection":   {Template: "http-api", Target: "aws", Name: "demo-svc", Params: map[string]string{"aws_region": "x\"\n}"}},
	}
	for name, req := range cases {
		if _, err := Render(c, req); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestSearchRanksByRelevance(t *testing.T) {
	c := load(t)
	got := c.Search("queue worker", "")
	if len(got) == 0 || got[0].ID != "queue-worker" {
		t.Fatalf("expected queue-worker first, got %v", got)
	}
	if n := len(c.Search("zzzz-nothing", "")); n != 0 {
		t.Fatalf("expected no hits, got %d", n)
	}
}
