package main

import (
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"ops/internal/server/api"
	"ops/internal/server/config"
	"ops/internal/server/deploy"
	"ops/internal/server/store"
	webstatic "ops/web"
)

func main() {
	configPath := "config/server.yaml"
	if len(os.Args) > 2 && os.Args[1] == "--config" {
		configPath = os.Args[2]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load server config: %v", err)
	}

	jarStore, err := store.NewJarStore(cfg.JarDir)
	if err != nil {
		log.Fatalf("create jar store: %v", err)
	}
	recordStore, err := store.NewRecordStore(cfg.RecordFile)
	if err != nil {
		log.Fatalf("create record store: %v", err)
	}

	hub := deploy.NewAgentHub()
	manager, err := deploy.NewManager(jarStore, recordStore, hub, buildJarBaseURL(cfg.ListenAddr, cfg.PublicBaseURL))
	if err != nil {
		log.Fatalf("create manager: %v", err)
	}

	jarHandler := api.NewJarHandler(jarStore)
	deployHandler := api.NewDeployHandler(manager, hub)
	wsHandler := api.NewWSHandler(hub)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/jars", jarHandler.Upload)
	mux.HandleFunc("GET /api/jars", jarHandler.List)
	mux.HandleFunc("GET /api/jars/download/{filename}", jarHandler.Download)
	mux.HandleFunc("POST /api/deploy", deployHandler.CreateDeploy)
	mux.HandleFunc("GET /api/tasks/{task_id}", deployHandler.GetTask)
	mux.HandleFunc("GET /api/tasks", deployHandler.ListTasks)
	mux.HandleFunc("GET /api/agent/status", deployHandler.AgentStatus)
	mux.Handle(cfg.WSPath, wsHandler)
	mux.HandleFunc("/", serveIndex)

	log.Printf("server listening on %s", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}

func buildJarBaseURL(listenAddr, publicBaseURL string) string {
	if publicBaseURL != "" {
		return strings.TrimRight(publicBaseURL, "/") + "/api/jars/download"
	}
	host := listenAddr
	if strings.HasPrefix(host, ":") {
		host = "127.0.0.1" + host
	}
	if strings.HasPrefix(host, "0.0.0.0:") {
		host = "127.0.0.1" + strings.TrimPrefix(host, "0.0.0.0")
	}
	if strings.HasPrefix(host, "[::]:") {
		_, port, _ := net.SplitHostPort(host)
		host = net.JoinHostPort("127.0.0.1", port)
	}
	return (&url.URL{
		Scheme: "http",
		Host:   host,
		Path:   "/api/jars/download",
	}).String()
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") {
		http.NotFound(w, r)
		return
	}
	content, err := webstatic.Files.ReadFile("index.html")
	if err != nil {
		http.Error(w, "read embedded index failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(content)
}
