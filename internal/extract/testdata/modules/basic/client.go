package acme

import (
	"context"
	"errors"
	"io"

	"example.com/acme/internal/secret"
	"example.com/acme/option"
	yaml "gopkg.in/yaml.v3"
)

// Color is a color used by widgets.
type Color int

// Colors supported by widgets.
const (
	// Red is red.
	Red   Color = iota
	Green       // Green is green.
)

// DefaultTimeout is the default request timeout in seconds.
const DefaultTimeout = 30

// ErrNotFound is returned when a widget does not exist.
var ErrNotFound = errors.New("not found")

// Client calls the Acme API. Create one with [NewClient] and see [Client.Do].
//
// It is safe for **concurrent** use. See [the guide](https://acme.dev/guide), [io.Reader], [Missing], and `[Widget]`.
type Client struct {
	// BaseURL overrides the API endpoint.
	BaseURL string `json:"base_url"`

	// Widgets manages widgets.
	Widgets *WidgetService
	Options []option.RequestOption // Options apply to every request.
	// Legacy is kept for compatibility.
	//
	// Deprecated: Use [Client.Do] instead.
	Legacy string
	secret string
	io.Reader
}

// NewClient returns a [Client] configured with opts.
func NewClient(opts ...option.RequestOption) *Client {
	return &Client{Options: opts}
}

// Do sends a request and returns the resulting [Widget].
func (c *Client) Do(ctx context.Context, body io.Reader) (*Widget, error) {
	return nil, nil
}

// Token returns the client's secret token.
func (c *Client) Token() vault.Token {
	return ""
}

// WidgetService manages widgets.
//
// Deprecated: Use [Client] directly.
type WidgetService struct {
	client *Client
}

// Get fetches a widget by ID.
func (s *WidgetService) Get(ctx context.Context, id string) (*Widget, error) {
	return nil, nil
}

// Widget is a widget. Call [Widget.Fields] to list its field names.
type Widget struct {
	ID    string
	Color Color
	Meta  yaml.Node
}

// Fields lists the widget's field names.
func (w *Widget) Fields() []string {
	return nil
}

// Settings configures a request.
type Settings = struct {
	// Timeout is the request timeout in seconds.
	Timeout int
}

// Store persists widgets.
type Store interface {
	io.Closer
	// Save stores w.
	Save(ctx context.Context, w *Widget) error
	load() error
}

// Handler handles a widget.
type Handler func(w *Widget) error

// Alias refers to [Widget].
type Alias = Widget

// Parse parses a widget.
//
// # Format
//
// The input looks like:
//
//	id=1 color=red
func Parse(input string) (Widget, error) {
	return Widget{}, nil
}

func helper() {}
