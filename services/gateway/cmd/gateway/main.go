// Dobra Gateway Agent — on-premise data gateway.
// Reads a YAML config, heartbeats to the central API, and exposes a local
// /tunnel/query endpoint that forwards SQL queries to configured sources.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/thedobra/thedobra/services/gateway/internal/gateway"
)

type Config struct {
	RemoteURL string          `json:"remote_url" yaml:"remote_url"`
	Token     string          `json:"token" yaml:"token"`
	Instance  string          `json:"instance" yaml:"instance"`
	Listen    string          `json:"listen" yaml:"listen"`
	Sources   []gateway.Source `json:"sources" yaml:"sources"`
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	if path == "" {
		path = "gateway.yaml"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		// Fallback to YAML if json fails; for MVP require JSON or env.
		return cfg, fmt.Errorf("parse config: %w", err)
	}
	if cfg.RemoteURL == "" {
		cfg.RemoteURL = os.Getenv("DOBRA_REMOTE")
	}
	if cfg.Token == "" {
		cfg.Token = os.Getenv("DOBRA_TOKEN")
	}
	if cfg.Instance == "" {
		cfg.Instance = hostname()
	}
	if cfg.Listen == "" {
		cfg.Listen = ":9000"
	}
	return cfg, nil
}

func hostname() string {
	h, _ := os.Hostname()
	if h == "" {
		return "gateway-local"
	}
	return h
}

func main() {
	path := os.Getenv("DOBRA_GATEWAY_CONFIG")
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	cfg, err := loadConfig(path)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if cfg.RemoteURL == "" || cfg.Token == "" {
		log.Fatal("DOBRA_REMOTE and DOBRA_TOKEN are required")
	}

	agent := gateway.NewAgent(cfg.RemoteURL, cfg.Token, cfg.Instance, cfg.Sources)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go heartbeatLoop(ctx, cfg, agent)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		gateway.WriteJSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/tunnel/query", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			gateway.WriteJSON(w, 405, map[string]string{"error": "method not allowed"})
			return
		}
		var req gateway.QueryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			gateway.WriteJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		res, err := agent.Query(r.Context(), req)
		if err != nil {
			gateway.WriteJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		gateway.WriteJSON(w, 200, res)
	})

	log.Printf("gateway listening on %s", cfg.Listen)
	if err := http.ListenAndServe(cfg.Listen, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func heartbeatLoop(ctx context.Context, cfg Config, agent *gateway.Agent) {
	url := cfg.RemoteURL + "/gateway/heartbeat"
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	beat := func() {
		payload, _ := json.Marshal(map[string]any{
			"token":   cfg.Token,
			"name":    cfg.Instance,
			"version": "0.1.0",
			"status":  "online",
			"metadata": map[string]any{
				"sources": sourceNames(cfg.Sources),
			},
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			log.Printf("heartbeat build: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("heartbeat failed: %v", err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			log.Printf("heartbeat rejected: %d", resp.StatusCode)
			return
		}
		log.Printf("heartbeat sent: %d", resp.StatusCode)
	}

	beat()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			beat()
		}
	}
}

func sourceNames(srcs []gateway.Source) []string {
	out := make([]string, len(srcs))
	for i, s := range srcs {
		out[i] = s.Name
	}
	return out
}
