// SDK Modification Suggestion
// Add this to the 1money-go-sdk/1money.go file to support custom URLs
// This file is for reference only and should not be compiled with the load_runner

// +build ignore

package onemoney

// Add this option function to support custom base URLs
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		// Note: You'll need to make baseHost a field in the Client struct
		// Currently it's only used during initialization
		c.baseHost = url
	}
}

// Alternative approach: Add a new constructor
func NewClientWithURL(url string, opts ...ClientOption) *Client {
	// This would require exporting the newClientInternal function
	// or duplicating its logic here
	return newClientInternal(url, opts...)
}

// Or add a Config-based approach:
type Config struct {
	ApiUrl   string
	Timeout  time.Duration
	Logger   Logger
	Hooks    []Hook
}

func NewClientWithConfig(config *Config, opts ...ClientOption) *Client {
	// Apply config settings
	if config.Timeout > 0 {
		opts = append(opts, WithTimeout(config.Timeout))
	}
	if config.Logger != nil {
		opts = append(opts, WithLogger(config.Logger))
	}
	if len(config.Hooks) > 0 {
		opts = append(opts, WithHooks(config.Hooks...))
	}
	
	// Use custom URL or default
	url := config.ApiUrl
	if url == "" {
		url = apiBaseHost
	}
	
	return newClientInternal(url, opts...)
}