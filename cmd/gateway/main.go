package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/gliderlab/cogate/gateway"
	"github.com/gliderlab/cogate/pkg/binddb"
	"github.com/gliderlab/cogate/pkg/config"
	"github.com/gliderlab/cogate/rpcproto"
	"github.com/gliderlab/cogate/storage"
)

type Config struct {
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseUrl"`
	Model   string `json:"model"`
	Port    int    `json:"port"`
	DBPath  string `json:"dbPath"`
}

func main() {
	// binddb: Check executable-database binding
	if dbPath, err := binddb.BindDB(); err != nil {
		log.Fatalf("binddb error: %v", err)
	} else if dbPath != "" {
		log.Printf("binddb: database bound at %s", dbPath)
	}

	log.Println("Starting OCG Gateway...")

	// Workdir/config/db defaults (relative to current cwd)
	cwd, _ := os.Getwd()
	if cwd == "" {
		cwd = "."
	}
	// Find directories - prefer binary location over CWD
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	workDir := filepath.Join(exeDir, "work")
	dbDir := filepath.Join(exeDir, "db")
	configDir := filepath.Join(exeDir, "config")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		// Fallback to CWD
		workDir = filepath.Join(cwd, "work")
		dbDir = filepath.Join(cwd, "db")
		configDir = filepath.Join(cwd, "config")
	}
	_ = os.MkdirAll(workDir, 0o755)
	_ = os.MkdirAll(dbDir, 0o755)
	_ = os.MkdirAll(configDir, 0o755)

	// Change to work directory
	if err := os.Chdir(workDir); err != nil {
		log.Printf("Warning: failed to change to work dir: %v", err)
	}

	envConfig := config.ReadEnvConfig(filepath.Join(configDir, "env.config"))

	// Parse bind host
	host := os.Getenv("OCG_HOST")
	if host == "" {
		host = envConfig["OCG_HOST"]
	}
	if host == "" {
		host = "127.0.0.1"
	}

	// Parse port
	port := os.Getenv("OCG_PORT")
	p, _ := strconv.Atoi(port)
	if p == 0 {
		if v, ok := envConfig["OCG_PORT"]; ok {
			p, _ = strconv.Atoi(v)
		}
	}

	// Fallback to config.json (optional)
	cfgFile := filepath.Join(configDir, "config.json")
	if p == 0 {
		if _, err := os.Stat(cfgFile); err == nil {
			data, _ := os.ReadFile(cfgFile)
			var c Config
			if err := json.Unmarshal(data, &c); err == nil {
				if c.Port > 0 {
					p = c.Port
				}
				log.Printf("Loaded port from config.json")
			}
		}
	}

	if p == 0 {
		p = config.DefaultGatewayPort
	}

	// Agent socket (Unix, no port)
	agentSock := os.Getenv("OCG_AGENT_SOCK")
	if agentSock == "" {
		agentSock = envConfig["OCG_AGENT_SOCK"]
	}
	if agentSock == "" {
		agentSock = config.DefaultSocketPath()
	}

	// 1) Connect to Agent (ocg-managed)
	client, err := rpcproto.DialAgent(agentSock, 20*time.Second)
	if err != nil {
		log.Printf("Failed to connect to Agent: %v", err)
		os.Exit(1)
	}
	defer client.Close()

	uiToken := os.Getenv("OCG_UI_TOKEN")
	if uiToken == "" {
		uiToken = envConfig["OCG_UI_TOKEN"]
	}

	srv := gateway.New(config.GatewayConfig{
		Host:        host,
		Port:        p,
		AgentAddr:   agentSock,
		UIAuthToken: uiToken,
	})
	srv.SetClient(client)

	// Initialize storage for rate limiting
	dbPath := envConfig["OCG_DB_PATH"]
	if dbPath == "" {
		dbPath = filepath.Join(dbDir, "ocg.db")
	}
	store, err := storage.New(dbPath)
	if err != nil {
		log.Printf("Failed to open storage: %v", err)
		client.Close()
		os.Exit(1)
	}
	defer store.Close()
	srv.SetStore(store)
	srv.SetWebhookStorage(store)

	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("Gateway start failed: %v", err)
			store.Close()
			client.Close()
			os.Exit(1)
		}
	}()

	log.Printf("Gateway listening on http://%s:%d", host, p)
	log.Println("Waiting for messages...")

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	log.Println("Gateway shutting down...")
	srv.Stop()
}
