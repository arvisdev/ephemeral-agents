// Package dockercli wraps the Docker CLI (the container image ships the
// docker client and a rw /var/run/docker.sock mount — no docker-in-docker).
// Every operation mirrors the exact commands pinned in
// M2-Lifecycle-Installation-and-Deployment.md §1b: docker run -d,
// docker ps -a --filter label=..., docker rm -f.
package dockercli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ephemeral-agents/poc/services/lifecycle/internal/reap"
)

// CLI executes docker commands with a per-command timeout.
type CLI struct {
	Bin     string // default "docker"
	Timeout time.Duration
}

func New() *CLI { return &CLI{Bin: "docker", Timeout: 60 * time.Second} }

var ErrNotFound = errors.New("container not found")

// run executes docker args, returning stdout; not-found is ErrNotFound.
func (c *CLI) run(ctx context.Context, args ...string) (string, error) {
	bin := c.Bin
	if bin == "" {
		bin = "docker"
	}
	t := c.Timeout
	if t <= 0 {
		t = 60 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()
	cmd := exec.CommandContext(cctx, bin, args...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if strings.Contains(msg, "No such") || strings.Contains(msg, "not found") {
			return "", ErrNotFound
		}
		return out.String(), fmt.Errorf("docker %s: %v: %s", args[0], err, msg)
	}
	return out.String(), nil
}

// ImagePresent implements the no-pull deploy gate / spawn-time check.
func (c *CLI) ImagePresent(ctx context.Context, image string) error {
	_, err := c.run(ctx, "image", "inspect", image)
	return err
}

// RunSpec is one docker run -d invocation.
type RunSpec struct {
	TaskID   string
	Image    string
	Env      []string // "KEY=value"
	WorkDir  string   // host dir bound to /work
	Expires  time.Time
	CPUs     float64 // 0 = unset
	MemBytes int64   // 0 = unset
}

// Run creates and starts the ephemeral container; returns its ID.
func (c *CLI) Run(ctx context.Context, s RunSpec) (string, error) {
	args := []string{"run", "-d",
		"--name", "poc-" + s.TaskID,
		"--label", reap.OwnerLabel + "=" + reap.OwnerValue,
		"--label", reap.TaskLabel + "=" + s.TaskID,
		"--label", reap.ExpiresLabel + "=" + s.Expires.UTC().Format(time.RFC3339),
		"-v", s.WorkDir + ":/work",
	}
	if s.CPUs > 0 {
		args = append(args, "--cpus", strconv.FormatFloat(s.CPUs, 'f', -1, 64))
	}
	if s.MemBytes > 0 {
		args = append(args, "--memory", strconv.FormatInt(s.MemBytes, 10)+"b")
	}
	for _, e := range s.Env {
		args = append(args, "-e", e)
	}
	args = append(args, s.Image)
	out, err := c.run(ctx, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// State describes one task's container (by its deterministic name).
type State struct {
	ContainerID string
	Status      string // created|running|exited|...
	ExitCode    int
	Expires     string
}

// Inspect resolves the container for a task; ErrNotFound when absent
// (reaped containers are normal, not errors).
func (c *CLI) Inspect(ctx context.Context, taskID string) (State, error) {
	// List by owner+task labels (exact-match filter, per the troubleshooting table).
	ids, err := c.run(ctx, "ps", "-aq",
		"--filter", reap.OwnerLabel+"="+reap.OwnerValue,
		"--filter", reap.TaskLabel+"="+taskID)
	if err != nil {
		return State{}, err
	}
	id := strings.TrimSpace(ids)
	if id == "" {
		return State{}, ErrNotFound
	}
	out, err := c.run(ctx, "inspect", "--format",
		`{{.Id}} {{.State.Status}} {{.State.ExitCode}} {{index .Config.Labels "`+reap.ExpiresLabel+`"}}`, id)
	if err != nil {
		return State{}, err
	}
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 4 {
		return State{}, fmt.Errorf("unexpected inspect output %q", out)
	}
	code, _ := strconv.Atoi(fields[2])
	return State{ContainerID: fields[0], Status: fields[1], ExitCode: code, Expires: fields[3]}, nil
}

// Expired lists POC-owned container IDs whose labels say the reaper must reap.
// The expiry DECISION is pure logic (internal/reap); this just feeds it.
func (c *CLI) ExpiredList(ctx context.Context, now time.Time) (ids []string, tasks []string, reasons []string, err error) {
	out, err := c.run(ctx, "ps", "-aq", "--filter", reap.OwnerLabel+"="+reap.OwnerValue)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, id := range strings.Fields(out) {
		insp, ierr := c.run(ctx, "inspect", "--format",
			`{{index .Config.Labels "`+reap.TaskLabel+`"}} {{index .Config.Labels "`+reap.ExpiresLabel+`"}}`, id)
		if ierr != nil {
			continue
		}
		f := strings.Fields(strings.TrimSpace(insp))
		task := ""
		if len(f) > 0 {
			task = f[0]
		}
		labels := map[string]string{}
		if len(f) > 1 {
			labels[reap.ExpiresLabel] = f[1]
		}
		if ok, reason := shouldReap(labels, now); ok {
			ids = append(ids, id)
			tasks = append(tasks, task)
			reasons = append(reasons, reason)
		}
	}
	return ids, tasks, reasons, nil
}

// shouldReap is an alias keeping the pure decision import local and obvious.
var shouldReap = reap.ShouldReap

// Logs captures stdout+stderr to dest (the per-task log file).
func (c *CLI) Logs(ctx context.Context, containerID, dest string) error {
	out, err := c.run(ctx, "logs", containerID)
	if err != nil && out == "" {
		return err
	}
	return os.WriteFile(dest, []byte(out), 0o644)
}

// Remove is `docker rm -f` (graceful stop then kill inside the CLI); absent
// container is a no-op success (idempotent reap).
func (c *CLI) Remove(ctx context.Context, taskID string) error {
	_, err := c.run(ctx, "rm", "-f", "poc-"+taskID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}
