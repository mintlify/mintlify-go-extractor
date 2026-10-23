package option

// Config holds request settings.
type Config struct {
	APIKey string
}

// RequestOption configures a request.
type RequestOption func(*Config)

// WithAPIKey sets the API key.
func WithAPIKey(key string) RequestOption {
	return func(c *Config) { c.APIKey = key }
}
