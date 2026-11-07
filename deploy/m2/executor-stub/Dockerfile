# M2 — stub executor image (lifecycle mechanics with zero agent logic)
# Source: version 1/installation-deployment/M2-Lifecycle-Installation-and-Deployment.md §1a
FROM debian:trixie-slim
RUN apt-get update && apt-get -y upgrade && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
