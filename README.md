# kickhub

**Description:** A template catalog that turns one choice (template + target) into a kickstart repository with application code, infrastructure and a local run path, for AWS, GCP or a fully local stack. Ships as a single Go binary with a CLI, a web UI and an MCP server.

**GitHub topics:** `golang` `scaffolding` `developer-portal` `platform-engineering` `terraform` `aws` `gcp` `docker-compose` `mcp` `templates`

## Why it exists

Internal developer portals usually offer a grid of approved patterns that create a repo for one cloud. kickhub keeps that model and changes four things:

| Typical portal | kickhub |
|---|---|
| One cloud per template | One template, three targets: `aws`, `gcp`, `local` |
| You see the repo after it is created | Preview every file before anything is created |
| Local run is left to the developer | The `local` target runs with `docker compose`, no cloud account |
| The repo forgets where it came from | `.kickhub.yaml` records template, version and parameters |

## Quick start

```sh
go build -o kickhub .
./kickhub list
./kickhub show queue-worker
./kickhub new queue-worker -name orders-worker -target local
cd orders-worker && make up && make send MSG=hello && make logs
```

Web UI and API:

```sh
./kickhub serve -addr :8080      # open http://localhost:8080
```

For coding agents (MCP over stdio), register the command `kickhub mcp`. It exposes `list_templates` and `create_component`.

## Built-in templates

| Template | AWS | GCP | Local |
|---|---|---|---|
| `http-api` | App Runner + DynamoDB + ECR | Cloud Run + Firestore + Artifact Registry | DynamoDB Local |
| `queue-worker` | SQS + dead-letter queue | Pub/Sub + dead-letter topic | ElasticMQ |
| `object-store` | S3 | Cloud Storage | MinIO |

## How a template works

```
templates/queue-worker/
  template.yaml          id, parameters, targets, resources, required tools
  base/                  files every target gets
  shared/sqs/            a layer several targets reuse (here: aws and local)
  targets/aws/           infra/main.tf
  targets/gcp/           infra/main.tf + the Pub/Sub adapter
  targets/local/         docker-compose.yml
```

A target is rendered as `base` → its `layers` → `targets/<name>`, later files winning. Application code depends on an interface (`Store`, `Queue`); only `provider.go` differs per target. That is why the local target exercises the same adapter code that runs on AWS.

Template files use `[[ ]]` delimiters so they never clash with GitHub Actions `${{ }}`, Terraform `${ }` or Helm `{{ }}`. Every file ends in `.tmpl`, which is stripped on output.

To add a template, copy a directory, edit `template.yaml`, and add it to `templates/embed.go`. Or keep your catalog outside the binary:

```sh
KICKHUB_TEMPLATES=/path/to/my/templates ./kickhub list
```

`go test ./...` renders every template for every target and fails on any unrendered syntax, so a broken template cannot ship.

## Commands

| Command | Purpose |
|---|---|
| `list [-q text] [-target t]` | Search the catalog |
| `show <template>` | Targets, resources and parameters |
| `new <template> -name n -target t [-out dir] [-set k=v]` | Create the repo |
| `doctor <template> -target t` | Check required CLI tools are installed |
| `serve [-addr :8080]` | Web UI + JSON API |
| `mcp` | MCP server on stdio |

## Known limits

- Application templates are Go only.
- `new` writes a directory; it does not create the GitHub repository or push.
- The local target uses AWS-compatible emulators. The GCP adapters honor the standard emulator environment variables, but no emulator compose file is generated for them.
- Local container images use `latest` tags; pin them for a team setup.
- `serve` has no authentication. Run it on localhost or behind your own proxy.

## Skills demonstrated

- Go: `embed`, `text/template`, `net/http` routing, table-driven tests, zero third-party dependencies beyond a YAML parser
- Platform engineering: golden-path scaffolding, layered template composition, provenance lockfiles
- Multi-cloud infrastructure as code: Terraform for AWS and GCP with least-privilege identities
- Ports-and-adapters design so one code base runs on three targets
- Model Context Protocol server implementation
