# M3 — Claude Code coder executor image
# Source: version 1/installation-deployment/M3-Executors-Installation-and-Deployment.md §2b
# pin minor
FROM node:22-slim

RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates procps && rm -rf /var/lib/apt/lists/*
# mandatory build-arg: exact CLI version (build fails if unset)
ARG CLAUDE_CODE_VERSION
RUN test -n "$CLAUDE_CODE_VERSION" && npm install -g @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
