// Gateway Tool - Common types and options
// Shared between Windows and Unix implementations

package tools

// GatewayToolOption configures the gateway tool
type GatewayToolOption func(*GatewayTool)

// WithGatewayURL sets the gateway URL
func WithGatewayURL(url string) GatewayToolOption {
	return func(t *GatewayTool) {
		t.gatewayURL = url
	}
}

// WithGatewayToken sets the gateway token
func WithGatewayToken(token string) GatewayToolOption {
	return func(t *GatewayTool) {
		t.gatewayToken = token
	}
}

// WithWorkDir sets the work directory
func WithWorkDir(dir string) GatewayToolOption {
	return func(t *GatewayTool) {
		t.workDir = dir
	}
}

// WithBinaryPath sets the binary path
func WithBinaryPath(path string) GatewayToolOption {
	return func(t *GatewayTool) {
		t.binaryPath = path
	}
}

// WithConfigPath sets the config path
func WithConfigPath(path string) GatewayToolOption {
	return func(t *GatewayTool) {
		t.configPath = path
	}
}
