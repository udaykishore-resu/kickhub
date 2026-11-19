# kickhub

**Description:** Template catalog that turns one choice (template + target) into a kickstart repo with code, Terraform and a local run path for AWS, GCP or docker compose. One Go binary: CLI, web UI and MCP server.

**GitHub topics:** `golang` `scaffolding` `developer-portal` `platform-engineering` `internal-developer-platform` `terraform` `aws` `gcp` `docker-compose` `mcp` `templates` `golden-path`

## Why it exists

Internal developer portals usually offer a grid of approved patterns that each create a repo for one cloud. kickhub keeps that model and changes six things:

| Typical portal | kickhub |
|---|---|
| One cloud per template | One template, up to three targets: `aws`, `gcp`, `local` |
| You see the repo after it is created | Preview every file before anything is created |
| Local run is left to the developer | The `local` target runs with `docker compose`, no cloud account |
| The repo forgets where it came from | `.kickhub.yaml` records template, version and parameters |
| Nobody knows which repos drifted | `kickhub status` shows what changed since generation |
| New repos only | `-into` adds a component to an existing repo and never overwrites |

## Quick start

```sh
go build -o kickhub .
./kickhub list
./kickhub show queue-worker
./kickhub new queue-worker -name orders-worker -target local -out /tmp/orders-worker
cd /tmp/orders-worker && make up && make send MSG=hello && make logs
```

Web UI and API:

```sh
./kickhub serve -addr :8080      # open http://localhost:8080
```

For coding agents (MCP over stdio), register the command `kickhub mcp`. It exposes `list_templates` and `create_component`.

## Catalog

20 templates. A blank cell means that target does not exist for that template, on purpose.

| Category | Template | AWS | GCP | Local |
|---|---|---|---|---|
| Basic | `basic-component` | any cloud: README, CODEOWNERS, CI, decision log | | |
| Basic | `ci-onboarding` | any cloud: GitHub Actions, Jenkinsfile, Dependabot | | |
| Serverless | `function-api` | Lambda + function URL | Cloud Functions 2nd gen | container |
| Serverless | `bucket-function` | S3 → Lambda | Cloud Storage → Cloud Function | MinIO → webhook |
| Containers | `http-api` | App Runner + DynamoDB | Cloud Run + Firestore | DynamoDB Local |
| Containers | `queue-worker` | SQS + dead-letter queue | Pub/Sub + dead-letter topic | ElasticMQ |
| Batch Processing | `scheduled-job` | Lambda + EventBridge Scheduler | Cloud Run job + Cloud Scheduler | container |
| Batch Processing | `batch-job` | ECS Fargate task | Cloud Run job | container |
| Databases | `postgres-db` | Aurora Serverless v2 | Cloud SQL | PostgreSQL 16 |
| Databases | `redis-cache` | ElastiCache | Memorystore | Redis 7 |
| Databases | `search-cluster` | OpenSearch Service | | OpenSearch + Dashboards |
| Databases | `graph-db` | Neptune Serverless | | Gremlin Server |
| Individual Resources | `object-store` | S3 | Cloud Storage | MinIO |
| Individual Resources | `topic` | SNS | Pub/Sub topic | Pub/Sub emulator |
| Individual Resources | `queue` | SQS + dead-letter queue | Pub/Sub + dead-letter topic | ElasticMQ |
| Individual Resources | `firewall` | Security group | VPC firewall rule | |
| ML Model | `model-api` | App Runner (FastAPI) | Cloud Run (FastAPI) | container |
| Other | `go-library` | any cloud: publishable Go module | | |
| Other | `docs-site` | any cloud: MkDocs site on GitHub Pages | | |
| Other | `shared-config` | S3 state bucket + lock table | Cloud Storage state bucket | |

## Existing repos and drift

```sh
# Add CI to a repo that already exists. Files already there are kept and listed.
./kickhub new ci-onboarding -name my-service -into /path/to/my-service

# Later: what did people change, and is the template newer than the repo?
./kickhub status /path/to/my-service
```

`status` re-renders the template recorded in `.kickhub.yaml` and reports each generated file as unchanged, modified or missing.

## How a template works

```
templates/
  _shared/                 layers any template can reuse
    go/                    go.mod, CI workflow, .gitignore
    go-docker/             Dockerfile
    tf-aws/  tf-gcp/       provider blocks and naming variables
    infra-only/            Makefile and README for resource-only templates
  queue-worker/
    template.yaml          id, parameters, targets, resources, required tools
    base/                  files every target gets
    shared/sqs/            a layer two of its targets reuse (aws and local)
    targets/aws/           infra/main.tf
    targets/gcp/           infra/main.tf + the Pub/Sub adapter
    targets/local/         docker-compose.yml
```

A target is rendered as `_shared` layers → `base` → the template's own layers → `targets/<name>`, later files winning. Application code depends on an interface (`Store`, `Queue`) or a plain function (`Process`, `Run`); only the adapter or entry point differs per target. That is why the local target exercises the same code that runs in the cloud.

Rules a template author needs:

- Files use `[[ ]]` delimiters so they never clash with GitHub Actions `${{ }}`, Terraform `${ }` or Helm `{{ }}`.
- Every file ends in `.tmpl`, which is stripped on output. File names are templates too (`[[.P.package]].go.tmpl`).
- A file that renders empty is left out, which is how optional files work.
- A resource-only template is a `template.yaml` plus one file per target; `_shared/infra-only` supplies the rest.
- A template whose output does not depend on a cloud declares the single target `any`.

A new directory under `templates/` is picked up automatically. Or keep a catalog outside the binary:

```sh
KICKHUB_TEMPLATES=/path/to/my/templates ./kickhub list
```

## Testing

```sh
go test ./...        # renders every template for every target; fails on unrendered syntax
scripts/verify.sh    # generates all 48 template and target combinations and runs the real tools on each
```

`verify.sh` runs `go vet`, `go test`, `gofmt`, a Lambda cross-build, `terraform fmt` and `validate`, `docker compose config`, `pytest` and `mkdocs build --strict`, depending on what each generated repo contains.

## Commands

| Command | Purpose |
|---|---|
| `list [-q text] [-target t]` | Search the catalog |
| `show <template>` | Targets, resources and parameters |
| `new <template> -name n -target t [-out dir] [-set k=v]` | Create the repo |
| `new <template> -name n -into dir` | Add to an existing repo without overwriting |
| `status [dir]` | Drift since generation |
| `doctor <template> -target t` | Check required CLI tools are installed |
| `serve [-addr :8080]` | Web UI + JSON API |
| `mcp` | MCP server on stdio |

## Known limits

- Application templates are Go, plus one Python template (`model-api`).
- `new` writes a directory; it does not create the GitHub repository or push.
- `status` reports drift but does not merge a newer template version into a repo.
- Local targets use emulators with `latest` image tags; pin them for a team setup.
- `serve` has no authentication. Run it on localhost or behind your own proxy.
- Database templates on AWS need an existing VPC and subnets; they do not create a network.

## Skills demonstrated

- Go: `embed`, `text/template`, `net/http` routing, table-driven tests, one third-party dependency (a YAML parser)
- Platform engineering: golden-path scaffolding, layered template composition, provenance lockfiles, drift detection
- Multi-cloud infrastructure as code: Terraform for AWS and GCP with least-privilege identities
- Serverless and container runtimes: Lambda, Cloud Functions, App Runner, Cloud Run, ECS Fargate
- Ports-and-adapters design so one code base runs on three targets
- Model Context Protocol server implementation
