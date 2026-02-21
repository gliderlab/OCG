// Package di provides dependency injection for OCG services
// Replaces hardcoded values with configurable dependencies
package di

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/gliderlab/cogate/pkg/config"
)

// ConfigLoader loads configuration from various sources
type ConfigLoader interface {
	Load(path string) map[string]string
}

// Database opens database connections
type Database interface {
	Open(dsn string) (*sql.DB, error)
	OpenSQLite(dsn string) (*sql.DB, error)
}

// EmbeddingProvider generates vector embeddings
type EmbeddingProvider interface {
	Embed(text string) ([]float32, error)
	Dim() int
	Name() string
}

// Logger provides logging capabilities
type Logger interface {
	Print(v ...interface{})
	Printf(format string, v ...interface{})
}

// TimeProvider provides current time
type TimeProvider interface {
	Now() time.Time
	Sleep(duration time.Duration)
}

// IDGenerator generates unique IDs
type IDGenerator interface {
	New() string
}

// HTTPClient makes HTTP requests
type HTTPClient interface {
	Get(url string) (*http.Response, error)
	Do(req *http.Request) (*http.Response, error)
}

// ============ Default Implementations ============

type defaultConfigLoader struct{}

func (d *defaultConfigLoader) Load(path string) map[string]string {
	return config.ReadEnvConfig(path)
}

type defaultDatabase struct{}

func (d *defaultDatabase) Open(dsn string) (*sql.DB, error) {
	return sql.Open("sqlite3", dsn)
}

func (d *defaultDatabase) OpenSQLite(dsn string) (*sql.DB, error) {
	return sql.Open("sqlite3", dsn)
}

type defaultLogger struct{}

func (d *defaultLogger) Print(v ...interface{}) {
	log.Print(v...)
}

func (d *defaultLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

type defaultTimeProvider struct{}

func (d *defaultTimeProvider) Now() time.Time {
	return time.Now()
}

func (d *defaultTimeProvider) Sleep(duration time.Duration) {
	time.Sleep(duration)
}

type defaultIDGenerator struct{}

func (d *defaultIDGenerator) New() string {
	return generateID()
}

type defaultHTTPClient struct {
	client *http.Client // FIX: now used
}

func (d *defaultHTTPClient) Get(url string) (*http.Response, error) {
	// FIX: Use the client field with timeout instead of http.Get
	if d.client == nil {
		d.client = &http.Client{Timeout: 30 * time.Second}
	}
	return d.client.Get(url)
}

func (d *defaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// FIX: Use the client field
	if d.client == nil {
		d.client = &http.Client{Timeout: 30 * time.Second}
	}
	return d.client.Do(req)
}

// ============ Container ============

type Container struct {
	configLoader ConfigLoader
	database     Database
	logger       Logger
	timeProvider TimeProvider
	idGenerator  IDGenerator
	httpClient   HTTPClient
}

// NewContainer creates a new dependency injection container with defaults
func NewContainer() *Container {
	return &Container{
		configLoader: &defaultConfigLoader{},
		database:     &defaultDatabase{},
		logger:       &defaultLogger{},
		timeProvider: &defaultTimeProvider{},
		idGenerator:  &defaultIDGenerator{},
		httpClient:   &defaultHTTPClient{},
	}
}

// WithConfigLoader sets a custom config loader
func (c *Container) WithConfigLoader(loader ConfigLoader) *Container {
	c.configLoader = loader
	return c
}

// WithDatabase sets a custom database interface
func (c *Container) WithDatabase(db Database) *Container {
	c.database = db
	return c
}

// WithLogger sets a custom logger
func (c *Container) WithLogger(logger Logger) *Container {
	c.logger = logger
	return c
}

// WithTimeProvider sets a custom time provider
func (c *Container) WithTimeProvider(tp TimeProvider) *Container {
	c.timeProvider = tp
	return c
}

// WithIDGenerator sets a custom ID generator
func (c *Container) WithIDGenerator(gen IDGenerator) *Container {
	c.idGenerator = gen
	return c
}

// WithHTTPClient sets a custom HTTP client
func (c *Container) WithHTTPClient(client HTTPClient) *Container {
	c.httpClient = client
	return c
}

// Getters
func (c *Container) ConfigLoader() ConfigLoader { return c.configLoader }
func (c *Container) Database() Database         { return c.database }
func (c *Container) Logger() Logger             { return c.logger }
func (c *Container) TimeProvider() TimeProvider { return c.timeProvider }
func (c *Container) IDGenerator() IDGenerator   { return c.idGenerator }
func (c *Container) HTTPClient() HTTPClient     { return c.httpClient }

// Global container instance
var globalContainer *Container

// Global returns the global container (creates if needed)
func Global() *Container {
	if globalContainer == nil {
		globalContainer = NewContainer()
	}
	return globalContainer
}

// SetGlobal sets the global container
func SetGlobal(c *Container) {
	globalContainer = c
}

func generateID() string {
	return time.Now().Format("20060102150405.000000000")
}

// End of file
