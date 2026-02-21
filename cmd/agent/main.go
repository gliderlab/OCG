package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"google.golang.org/grpc"

	"github.com/gliderlab/cogate/agent"
	"github.com/gliderlab/cogate/memory"
	"github.com/gliderlab/cogate/pkg/binddb"
	pkgconfig "github.com/gliderlab/cogate/pkg/config"
	"github.com/gliderlab/cogate/pkg/kv"
	"github.com/gliderlab/cogate/rpcproto"
	"github.com/gliderlab/cogate/storage"
	"github.com/gliderlab/cogate/tools"
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

	log.Println("Starting OCG Agent...")

	// Default dirs - prefer binary location over CWD
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	cwd, _ := os.Getwd()
	if cwd == "" {
		cwd = "."
	}
	// Try binary dir first
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

	// 1. Read env.config (initial boot)
	envConfig := pkgconfig.ReadEnvConfig(filepath.Join(configDir, "env.config"))
	syncEnvToConfig(filepath.Join(configDir, "env.config"), envConfig, []string{
		"OCG_API_KEY",
		"OCG_BASE_URL",
		"OCG_MODEL",
		"OCG_DB_PATH",
		"OCG_PORT",
		"OPENAI_API_KEY",
		"EMBEDDING_SERVER_URL",
		"EMBEDDING_MODEL",
	})

	// 2. Init SQLite storage
	dbPath := filepath.Join(dbDir, "ocg.db")
	if v, ok := envConfig["OCG_DB_PATH"]; ok && v != "" {
		dbPath = v
	}
	if v := os.Getenv("OCG_DB_PATH"); v != "" {
		dbPath = v
	}

	store, err := storage.New(dbPath)
	if err != nil {
		log.Fatalf("Storage init failed: %v", err)
	}
	defer store.Close()

	// Init vector memory store (FAISS + local embedding)
	embeddingServer := envConfig["EMBEDDING_SERVER_URL"]
	if v := os.Getenv("EMBEDDING_SERVER_URL"); v != "" {
		embeddingServer = v
	}
	embeddingModel := envConfig["EMBEDDING_MODEL"]
	if v := os.Getenv("EMBEDDING_MODEL"); v != "" {
		embeddingModel = v
	}
	openaiKey := envConfig["OPENAI_API_KEY"]
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		openaiKey = v
	}

	hnswPath := envConfig["HNSW_PATH"]
	if v := os.Getenv("HNSW_PATH"); v != "" {
		hnswPath = v
	}
	if hnswPath == "" {
		hnswPath = filepath.Join(dbDir, "vector.index")
	}

	memoryStore, err := memory.NewVectorMemoryStore(dbPath, memory.Config{
		EmbeddingServer: embeddingServer,
		EmbeddingModel:  embeddingModel,
		ApiKey:          openaiKey,
		HNSWPath:        hnswPath,
	})
	if err != nil {
		log.Printf("Vector memory init failed: %v", err)
	}
	if memoryStore != nil {
		defer memoryStore.Close()
	}

	// 3. Load config (skip file if DB already has config)
	var cfg pkgconfig.AgentConfig
	configExists, _ := store.ConfigExists("llm")

	forceEnvConfig := strings.ToLower(os.Getenv("OCG_FORCE_ENV_CONFIG")) == "true"
	if forceEnvConfig {
		configExists = false
		log.Printf("[Config] Force loading from env.config (OCG_FORCE_ENV_CONFIG=true)")
	}

	if !configExists {
		// 3.1 env.config
		if v, ok := envConfig["OCG_API_KEY"]; ok && v != "" {
			cfg.APIKey = v
		}
		if v, ok := envConfig["OCG_BASE_URL"]; ok && v != "" {
			cfg.BaseURL = v
		}
		if v, ok := envConfig["OCG_MODEL"]; ok && v != "" {
			cfg.Model = v
		}

		// 3.2 environment overrides
		if v := os.Getenv("OCG_API_KEY"); v != "" {
			cfg.APIKey = v
		}
		if v := os.Getenv("OCG_BASE_URL"); v != "" {
			cfg.BaseURL = v
		}
		if v := os.Getenv("OCG_MODEL"); v != "" {
			cfg.Model = v
		}

		// 3.3 optional config.json
		cfgFile := "config.json"
		if _, err := os.Stat(cfgFile); err == nil {
			data, err := os.ReadFile(cfgFile)
			if err != nil {
				log.Printf("Failed to read config.json: %v", err)
			} else {
				var c Config
				if err := json.Unmarshal(data, &c); err != nil {
					log.Printf("Failed to parse config.json: %v", err)
				} else {
					if c.APIKey != "" {
						cfg.APIKey = c.APIKey
					}
					if c.BaseURL != "" {
						cfg.BaseURL = c.BaseURL
					}
					if c.Model != "" {
						cfg.Model = c.Model
					}
					if c.Port > 0 {
						os.Setenv("OCG_PORT", fmt.Sprintf("%d", c.Port))
					}
					log.Printf("Loaded config from config.json")
				}
			}
		}
	} else {
		log.Printf("Config found in database, skipping file load")
	}

	autoRecall := strings.ToLower(os.Getenv("OCG_AUTO_RECALL"))
	if autoRecall == "" {
		autoRecall = strings.ToLower(envConfig["OCG_AUTO_RECALL"])
	}
	log.Printf("Config: API Key=%s, BaseURL=%s, Model=%s, DB=%s, AutoRecall=%v",
		maskKey(cfg.APIKey), cfg.BaseURL, cfg.Model, dbPath, autoRecall == "true")

	// 4. Init Agent with storage
	var registry *tools.Registry
	if memoryStore != nil {
		registry = tools.NewMemoryRegistry(memoryStore)
	} else {
		registry = tools.NewDefaultRegistry()
	}

	recallLimit := 3
	if v := os.Getenv("OCG_RECALL_LIMIT"); v != "" {
		fmt.Sscanf(v, "%d", &recallLimit)
	}
	if recallLimit <= 0 {
		recallLimit = 3
	}
	recallMinScore := 0.3
	if v := os.Getenv("OCG_RECALL_MINSCORE"); v != "" {
		fmt.Sscanf(v, "%f", &recallMinScore)
	}
	if recallMinScore <= 0 {
		recallMinScore = 0.3
	}

	cfg.AutoRecall = strings.ToLower(autoRecall) == "true"
	cfg.RecallLimit = recallLimit
	cfg.RecallMinScore = recallMinScore
	cfg.PulseEnabled = true

	// Parse models config for context window fallback
	if v := os.Getenv("OCG_MODELS"); v != "" {
		if err := json.Unmarshal([]byte(v), &cfg.Models); err != nil {
			log.Printf("Failed to parse OCG_MODELS: %v", err)
		}
	}

	// 4.5 Init KV store (BadgerDB for fast caching)
	// Default: in-memory mode. Use OCG_KV_DIR to enable persistence
	var kvStore *kv.KV
	kvDir := os.Getenv("OCG_KV_DIR")
	if kvDir != "" {
		// Persistent mode: use file storage
		kvStore, err = kv.Open(kv.Options{
			Dir:           kvDir,
			Compression:   true,
			ValueLogMaxMB: 256,
		})
		if err != nil {
			log.Printf("KV store (persistent) init failed: %v (continuing without KV)", err)
		} else {
			log.Printf("KV store initialized (persistent): %s", kvDir)
		}
	} else {
		// In-memory mode (default) - no Dir allowed
		kvStore, err = kv.Open(kv.Options{
			MemoryMode: true,
			Dir:       "", // Must be empty for memory mode
		})
		if err != nil {
			log.Printf("KV store (memory) init failed: %v (continuing without KV)", err)
		} else {
			log.Printf("KV store initialized (in-memory)")
		}
	}
	if kvStore != nil {
		defer kvStore.Close()
	}

	// 5. Init Agent with storage
	ai := agent.New(agent.Config{
		AgentConfig: cfg,
		Storage:     store,
		MemoryStore: memoryStore,
		Registry:    registry,
	})

	// Set KV store to agent
	if kvStore != nil {
		ai.SetKV(kvStore)
	}

	// 6. Start RPC service (Unix socket, no port)
	sockPath := os.Getenv("OCG_AGENT_SOCK")
	if sockPath == "" {
		sockPath = envConfig["OCG_AGENT_SOCK"]
	}
	if sockPath == "" {
		sockPath = pkgconfig.DefaultSocketPath()
	}

	// Ensure old socket is removed
	_ = os.Remove(sockPath)

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		log.Printf("RPC listen failed: %v", err)
		return
	}
	defer listener.Close()
	_ = os.Chmod(sockPath, 0666)

	grpcServer := grpc.NewServer()
	rpcproto.RegisterAgentServer(grpcServer, agent.NewGRPCService(ai))

	log.Printf("Agent gRPC listening on unix://%s", sockPath)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Printf("gRPC serve error: %v", err)
		}
	}()

	// 6. Print storage stats
	if stats, err := store.Stats(); err == nil {
		log.Printf("Storage stats: %+v", stats)
	}

	// 7. Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	log.Println("Agent shutting down...")
	
	// Stop agent (includes stopping pulse/heartbeat goroutines)
	ai.Stop()
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func syncEnvToConfig(path string, config map[string]string, keys []string) {
	changed := false
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			if config[k] != v {
				config[k] = v
				changed = true
			}
		}
	}
	if changed {
		_ = pkgconfig.WriteEnvConfig(path, config)
	}
}
