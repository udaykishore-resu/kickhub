package engine

import (
	"bytes"
	"os"
	"path/filepath"
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
				params := map[string]string{}
				for _, p := range tpl.Parameters {
					if p.Required {
						params[p.Name] = "demo-project"
					}
				}
				res, err := Render(c, Request{Template: tpl.ID, Target: target, Name: "demo-svc", Params: params})
				if err != nil {
					t.Fatal(err)
				}
				for _, want := range []string{".kickhub.yaml"} {
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
				switch target {
				case "aws", "gcp":
					if !hasInfra || hasCompose {
						t.Errorf("cloud target must have infra and no compose: infra=%v compose=%v", hasInfra, hasCompose)
					}
				case "local":
					if !hasCompose || hasInfra {
						t.Errorf("local target must have compose and no infra: infra=%v compose=%v", hasInfra, hasCompose)
					}
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

func TestMergeNeverOverwritesAndStatusSeesDrift(t *testing.T) {
	c := load(t)
	dir := t.TempDir()
	res, err := Render(c, Request{Template: "queue", Target: "local", Name: "demo-svc"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := res.WriteDir(dir, false); err != nil {
		t.Fatal(err)
	}
	if _, err := res.WriteDir(dir, false); err == nil {
		t.Fatal("writing into a non-empty directory must fail without merge")
	}

	edited := []byte("# mine\n")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), edited, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "Makefile")); err != nil {
		t.Fatal(err)
	}

	d, err := Status(c, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Modified) != 1 || d.Modified[0] != "README.md" || len(d.Missing) != 1 || d.Missing[0] != "Makefile" {
		t.Fatalf("drift: modified=%v missing=%v", d.Modified, d.Missing)
	}

	skipped, err := res.WriteDir(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "README.md"))
	if !bytes.Equal(got, edited) {
		t.Fatal("merge overwrote an existing file")
	}
	if len(skipped) != len(res.Files)-1 {
		t.Fatalf("expected only the Makefile to be restored, skipped %d of %d", len(skipped), len(res.Files))
	}
}
