// Copyright 2026 A&R Visual Development Inc (arvisdev)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command lifecycle is the M2 lifecycle service: the compose-network HTTP API
// (spawn/status/reap/health/report) plus the label-based reaper backstop loop.
//
// Config (env, per deploy/m2/compose.yaml):
//
//	DOCKER_HOST=unix:///var/run/docker.sock
//	LISTEN=0.0.0.0:8090
//	REAPER_INTERVAL=30s
//	POC_TTL_DEFAULT=900s
//	POC_IMAGE=poc/executor-stub:1.0.0   (pinned; no pulls at activity time)
//	POC_TASKS_DIR=/var/poc/tasks
//	POC_CPU_LIMIT / POC_MEM_BYTES       (optional resource limits)
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ephemeral-agents/poc/services/lifecycle/internal/dockercli"
	lifecyclesvc "github.com/ephemeral-agents/poc/services/lifecycle/internal/service"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustDuration(key, def string) time.Duration {
	d, err := time.ParseDuration(envOr(key, def))
	if err != nil || d <= 0 {
		slog.Error("bad duration env", "key", key, "value", envOr(key, def))
		os.Exit(1)
	}
	return d
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	listen := envOr("LISTEN", "0.0.0.0:8090")
	image := envOr("POC_IMAGE", "poc/executor-stub:1.0.0")
	defaultTTL := mustDuration("POC_TTL_DEFAULT", "900s")
	reapInterval := mustDuration("REAPER_INTERVAL", "30s")
	reapGrace := mustDuration("REAPER_GRACE", "30s")
	tasksDir := envOr("POC_TASKS_DIR", "/var/poc/tasks")
	cpu, mem := parseLimits(log)
	cli := dockercli.New()

	svc := lifecyclesvc.New(cli, log, image, defaultTTL, reapInterval, reapGrace, tasksDir, cpu, mem)

	// Deploy gate: refuse to serve until the pinned image exists locally.
	ctx := context.Background()
	if err := svc.ImagePresent(ctx); err != nil {
		log.Error("no-pull deploy gate FAILED: pinned image absent; not serving", "image", image, "err", err)
		os.Exit(1)
	}
	log.Info("deploy gate passed", "image", image)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go svc.RunReaper(ctx)

	srv := &http.Server{Addr: listen, Handler: svc.Handler(), ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Info("lifecycle serving", "listen", listen, "ttl_default_s", int(defaultTTL/time.Second))
	select {
	case err := <-errCh:
		log.Error("http server exited", "err", err)
		os.Exit(1)
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}

func parseLimits(log *slog.Logger) (nanoCPUs float64, memBytes int64) {
	if v := os.Getenv("POC_CPU_LIMIT"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f <= 0 {
			log.Error("bad POC_CPU_LIMIT", "value", v)
			os.Exit(1)
		}
		nanoCPUs = f
	}
	if v := os.Getenv("POC_MEM_BYTES"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			log.Error("bad POC_MEM_BYTES", "value", v)
			os.Exit(1)
		}
		memBytes = n
	}
	return nanoCPUs, memBytes
}
