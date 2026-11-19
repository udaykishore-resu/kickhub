// kickhub turns a catalog template into a ready-to-run kickstart repository
// for AWS, GCP or a fully local stack.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/udaykishore-resu/kickhub/internal/catalog"
	"github.com/udaykishore-resu/kickhub/internal/engine"
	"github.com/udaykishore-resu/kickhub/internal/mcp"
	"github.com/udaykishore-resu/kickhub/internal/server"
	"github.com/udaykishore-resu/kickhub/templates"
)

const usage = `kickhub - template catalog that creates kickstart repos for AWS, GCP and local

Usage:
  kickhub list   [-q "queue worker"] [-target aws|gcp|local]
  kickhub show   <template>
  kickhub new    <template> -name my-svc -target local [-out dir] [-set key=value ...]
  kickhub new    <template> -name my-svc -into .   add to an existing repo, never overwriting
  kickhub status [dir]                             what drifted from the template since generation
  kickhub doctor <template> -target aws
  kickhub serve  [-addr :8080]          web UI + JSON API
  kickhub mcp                           MCP server on stdio for coding agents

Set KICKHUB_TEMPLATES=/path/to/dir to use your own catalog instead of the built-in one.
`

type setFlags map[string]string

func (s setFlags) String() string { return "" }
func (s setFlags) Set(v string) error {
	k, val, ok := strings.Cut(v, "=")
	if !ok {
		return fmt.Errorf("expected key=value, got %q", v)
	}
	s[k] = val
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func loadCatalog() (*catalog.Catalog, error) {
	var fsys fs.FS = templates.FS
	if dir := os.Getenv("KICKHUB_TEMPLATES"); dir != "" {
		fsys = os.DirFS(dir)
	}
	return catalog.Load(fsys)
}

// splitArg separates the leading positional argument (template id) from flags.
func splitArg(args []string) (string, []string, error) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", nil, fmt.Errorf("template id is required; run 'kickhub list'")
	}
	return args[0], args[1:], nil
}

func run(cmd string, args []string) error {
	if cmd == "help" || cmd == "-h" || cmd == "--help" {
		fmt.Print(usage)
		return nil
	}
	c, err := loadCatalog()
	if err != nil {
		return err
	}
	fl := flag.NewFlagSet(cmd, flag.ContinueOnError)

	switch cmd {
	case "list":
		q := fl.String("q", "", "free-text search")
		target := fl.String("target", "", "only templates supporting this target")
		if err := fl.Parse(args); err != nil {
			return err
		}
		for _, t := range c.Search(*q, *target) {
			fmt.Printf("%-16s %-24s %-34s [%s]\n", t.ID, t.Category, t.Name, strings.Join(targetNames(t), ", "))
		}

	case "show":
		id, _, err := splitArg(args)
		if err != nil {
			return err
		}
		t, ok := c.Get(id)
		if !ok {
			return fmt.Errorf("unknown template %q", id)
		}
		fmt.Printf("%s (%s) v%s\n%s\n\nTargets:\n", t.Name, t.ID, t.Version, t.Description)
		for _, name := range targetNames(t) {
			tg := t.Targets[name]
			fmt.Printf("  %-6s %s\n         resources: %s\n         requires:  %s\n", name, tg.Summary,
				strings.Join(tg.Resources, ", "), strings.Join(tg.Requires, ", "))
		}
		fmt.Println("\nParameters (-set key=value):")
		for _, p := range t.Parameters {
			scope := "all targets"
			if len(p.Targets) > 0 {
				scope = strings.Join(p.Targets, ", ")
			}
			req := ""
			if p.Required {
				req = " (required)"
			}
			fmt.Printf("  %-14s %s%s [%s] default: %q\n", p.Name, p.Description, req, scope, p.Default)
		}

	case "new":
		id, rest, err := splitArg(args)
		if err != nil {
			return err
		}
		name := fl.String("name", "", "component name, lowercase-with-dashes")
		target := fl.String("target", "local", "aws, gcp or local")
		out := fl.String("out", "", "output directory (default ./<name>)")
		into := fl.String("into", "", "existing repo to add files to; files already there are kept")
		sets := setFlags{}
		fl.Var(sets, "set", "template parameter as key=value (repeatable)")
		if err := fl.Parse(rest); err != nil {
			return err
		}
		res, err := engine.Render(c, engine.Request{Template: id, Target: *target, Name: *name, Params: sets})
		if err != nil {
			return err
		}
		if *out == "" {
			*out = *name
		}
		if *into != "" {
			*out = *into
		}
		skipped, err := res.WriteDir(*out, *into != "")
		if err != nil {
			return err
		}
		fmt.Printf("Created %s from %s (target %s): %d files written to %s\n", *name, id, res.Target, len(res.Files)-len(skipped), *out)
		for _, p := range skipped {
			fmt.Printf("  kept existing %s\n", p)
		}
		printChecks(res.Template, res.Target)
		fmt.Printf("Next: cd %s && cat README.md\n", *out)

	case "doctor":
		id, rest, err := splitArg(args)
		if err != nil {
			return err
		}
		target := fl.String("target", "local", "aws, gcp or local")
		if err := fl.Parse(rest); err != nil {
			return err
		}
		t, ok := c.Get(id)
		if !ok {
			return fmt.Errorf("unknown template %q", id)
		}
		if _, ok := t.ResolveTarget(*target); !ok {
			return fmt.Errorf("template %q does not support target %q", id, *target)
		}
		printChecks(t, *target)

	case "status":
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		d, err := engine.Status(c, dir)
		if err != nil {
			return err
		}
		fmt.Printf("%s: %s (target %s), generated from v%s, catalog has v%s\n",
			d.Request.Name, d.Request.Template, d.Request.Target, d.LockedVersion, d.CurrentVersion)
		fmt.Printf("  %d unchanged, %d modified, %d missing\n", d.Unchanged, len(d.Modified), len(d.Missing))
		for _, p := range d.Modified {
			fmt.Println("  modified", p)
		}
		for _, p := range d.Missing {
			fmt.Println("  missing ", p)
		}

	case "serve":
		addr := fl.String("addr", ":8080", "listen address")
		if err := fl.Parse(args); err != nil {
			return err
		}
		log.Printf("kickhub listening on %s", *addr)
		return http.ListenAndServe(*addr, server.New(c))

	case "mcp":
		return mcp.Serve(c, os.Stdin, os.Stdout)

	default:
		return fmt.Errorf("unknown command %q\n\n%s", cmd, usage)
	}
	return nil
}

func targetNames(t *catalog.Template) []string {
	var out []string
	for name := range t.Targets {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func printChecks(t *catalog.Template, target string) {
	checks, _ := engine.Doctor(t, target)
	for _, ch := range checks {
		mark := "ok     "
		if !ch.Found {
			mark = "MISSING"
		}
		fmt.Printf("  %s %s\n", mark, ch.Tool)
	}
}
