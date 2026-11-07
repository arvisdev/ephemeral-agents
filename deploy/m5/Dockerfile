# M5 — gateway/manager image (one multi-stage Go build; install.sh builds the
# same Dockerfile against services/gateway and services/manager)
# Source: version 1/installation-deployment/M5-End-to-End-Installation-and-Deployment.md §1
# (the doc does not print this Dockerfile inline — transcribed to the M1 Dockerfile pattern)
FROM golang:1.26 AS build
ARG MAIN
WORKDIR /src
COPY . .
RUN go build -o /out/svc "${MAIN:-.}"

FROM debian:trixie-slim
RUN apt-get update && apt-get -y upgrade && apt-get install -y ca-certificates git && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/svc /usr/local/bin/svc
ENTRYPOINT ["svc"]
