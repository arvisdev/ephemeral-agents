# M3 — Hermes ephemeral executor image
# Source: version 1/installation-deployment/M3-Executors-Installation-and-Deployment.md §2a
# or the estate's pinned base; record in image-tags.env
FROM debian:trixie-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates git curl gnupg procps python3 && rm -rf /var/lib/apt/lists/*
# Hermes runtime install (per the owner's Hermes packaging):
# HERMES_INSTALLER_URL is a mandatory build-arg: the estate's pinned Hermes
# installer URL+revision. The build FAILS if unset (no silent defaults).
ARG HERMES_INSTALLER_URL
RUN test -n "$HERMES_INSTALLER_URL" && curl -fsSL "$HERMES_INSTALLER_URL" | bash
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
