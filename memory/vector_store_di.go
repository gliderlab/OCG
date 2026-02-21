package memory

import "github.com/gliderlab/cogate/pkg/config"

// NewVectorMemoryStoreWithConfig creates a vector store using injected config
func NewVectorMemoryStoreWithConfig(cfg config.MemoryConfig) (*VectorMemoryStore, error) {
	memCfg := Config{
		ApiKey:          cfg.EmbeddingApiKey,
		EmbeddingModel:  cfg.EmbeddingModel,
		EmbeddingServer: cfg.EmbeddingServer,
		EmbeddingDim:    cfg.EmbeddingDim,
		MaxResults:      cfg.MaxResults,
		MinScore:        cfg.MinScore,
		HNSWPath:        cfg.HNSWPath,
		HybridEnabled:   cfg.HybridEnabled,
		VectorWeight:    cfg.VectorWeight,
		TextWeight:      cfg.TextWeight,
		CandidateMult:   cfg.CandidateMult,
		BatchSize:       cfg.BatchSize,
	}
	return NewVectorMemoryStore(cfg.DBPath, memCfg)
}
