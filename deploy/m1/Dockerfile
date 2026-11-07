# M1 — topic-bridge adapter image (static single-binary build)
# Source: version 1/installation-deployment/M1-Adapter-Installation-and-Deployment.md §2
FROM golang:1.26 AS build
WORKDIR /src
COPY . .
RUN go build -o /out/adapter ./cmd/adapter

FROM debian:trixie-slim
RUN apt-get update && apt-get -y upgrade && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/adapter /usr/local/bin/adapter
ENTRYPOINT ["adapter"]
