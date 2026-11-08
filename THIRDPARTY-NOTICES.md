# Third-Party Notices

This project is distributed as **source code only**. No third-party binaries
are included in this repository: at deployment time, the install scripts and
docker-compose files download each third-party component **from its upstream
source**, under that component's own license. The components and their
licenses are listed here for attribution and so that every operator knows
what the deployment procedure fetches on their behalf.

## Runtime components (downloaded from upstream at deployment)

| Component | Source | License |
|---|---|---|
| Debian (base images) | deb.debian.org | Debian Free Software Guidelines / various permissive licenses |
| RabbitMQ | rabbitmq.com | MPL-2.0 |
| Temporal Server | temporal.io | MIT |
| PostgreSQL + pgvector | postgresql.org / github.com/pgvector/pgvector | PostgreSQL License |
| Docker / Moby (installed per host) | docker.com / github.com/moby/moby | Apache-2.0 |
| Claude Code CLI (pinned in the coder executor image) | anthropic.com | Anthropic's own terms (proprietary; not redistributed here) |
| Hermes Agent runtime (pinned in the hermes executor image) | hermes-agent.nousresearch.com | Its upstream terms (not redistributed here) |

## Go modules (source dependencies; see `go.mod` / `go.sum` per service)

| Module | License |
|---|---|
| github.com/rabbitmq/amqp091-go | BSD-2-Clause |
| go.temporal.io/sdk, go.temporal.io/api | MIT |
| github.com/grpc-ecosystem/*, google.golang.org/grpc | Apache-2.0 / BSD-3-Clause |
| github.com/google/uuid | BSD-3-Clause |
| golang.org/x/* | BSD-3-Clause |
| github.com/stretchr/testify | MIT |

The complete, authoritative dependency list is the per-service `go.mod` /
`go.sum` pair in this repository; each module's license text is available at
its source.

## Note on distribution

Because this repository ships source and manifests only, the Apache-2.0
notice obligations for binary distribution (§4) are not triggered by this
repository itself. If you build and distribute the container images or
binaries, the licenses above travel with them, and at that point you should
include each component's license text and upstream NOTICE file in the
distribution.
