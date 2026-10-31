// Command gateway is the M5 Hermes Agent Gateway on vm-xa — the "tall narrow
// bar" in the diagram. The engineer's desktop talks ONLY to this service:
//
//	POST /                        {title, agent_capability, task_brief}
//	                              -> forwarded to the manager (MANAGER_API).
//	GET  /healthz                 liveness.
//	GET  /gateway/reports/{id}    proxies the manager's /reports/{id}
//	                              (the report contract the desktop polls).
//
// M5 doc §2: the gateway is a thin front-of-house proxy; it owns no task
// state — the manager is the persistent, human-facing component.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
)

const (
	envListen     = "LISTEN"      // default 0.0.0.0:8080 (M5 compose)
	envManagerAPI = "MANAGER_API" // default http://manager:8086 (M5 compose)
)

// taskRequest is the single JSON the gateway accepts (M5 doc §2).
type taskRequest struct {
	Title           string `json:"title"`
	AgentCapability string `json:"agent_capability"`
	TaskBrief       string `json:"task_brief"`
	TTLS            int64  `json:"ttl_s,omitempty"` // optional brief estimate; the manager owns the final ttl_s
}

var taskIDRe = regexp.MustCompile(`^[0-9a-fA-F-]{36}$`)

// validateRequest rejects the empty/oversized-field cases before the
// request leaves the boundary. Pure; unit-tested.
func validateRequest(r taskRequest) error {
	var missing []string
	if strings.TrimSpace(r.Title) == "" {
		missing = append(missing, "title")
	}
	if strings.TrimSpace(r.AgentCapability) == "" {
		missing = append(missing, "agent_capability")
	}
	if strings.TrimSpace(r.TaskBrief) == "" {
		missing = append(missing, "task_brief")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required field(s): %s", strings.Join(missing, ", "))
	}
	const maxBrief = 64 * 1024
	if len(r.TaskBrief) > maxBrief {
		return fmt.Errorf("task_brief exceeds %d bytes", maxBrief)
	}
	return nil
}

func newHandler(managerURL string) http.Handler {
	target, err := url.Parse(managerURL)
	if err != nil {
		log.Fatalf("bad manager URL %q: %v", managerURL, err)
	}
	proxy := &httputil.ReverseProxy{Rewrite: func(pr *httputil.ProxyRequest) {
		pr.SetURL(target)
		pr.Out.Host = target.Host
	}}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	// The engineer's single request path: validate the shape, then proxy
	// verbatim to the manager's POST /tasks.
	mux.HandleFunc("POST /", func(w http.ResponseWriter, r *http.Request) {
		var req taskRequest
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		if err := dec.Decode(&req); err != nil {
			httpError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
			return
		}
		if err := validateRequest(req); err != nil {
			httpError(w, http.StatusBadRequest, err.Error())
			return
		}
		body, _ := json.Marshal(req)
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))
		r.Header.Set("Content-Type", "application/json")
		r.URL.Path = "/tasks" // the manager's task-entry endpoint
		proxy.ServeHTTP(w, r)
	})

	// Report polling: GET /gateway/reports/<task_id> proxies the manager's
	// /reports/<task_id> (M5 §2 report contract).
	mux.HandleFunc("GET /gateway/reports/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/gateway/reports/")
		if !taskIDRe.MatchString(id) {
			httpError(w, http.StatusBadRequest, "malformed task_id")
			return
		}
		r.URL.Path = "/reports/" + id
		proxy.ServeHTTP(w, r)
	})

	return mux
}

func main() {
	listen := os.Getenv(envListen)
	if listen == "" {
		listen = "0.0.0.0:8080"
	}
	manager := os.Getenv(envManagerAPI)
	if manager == "" {
		manager = "http://manager:8086"
	}
	log.Printf("gateway listening on %s -> %s", listen, manager)
	if err := http.ListenAndServe(listen, newHandler(manager)); err != nil {
		log.Fatal(err)
	}
}

func httpError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
