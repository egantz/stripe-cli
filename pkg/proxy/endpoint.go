package proxy

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

//
// Public types
//

// EndpointConfig contains the optional configuration parameters of an EndpointClient.
type EndpointConfig struct {
	HTTPClient *http.Client

	Log *log.Logger

	ResponseHandler EndpointResponseHandler
}

// EndpointResponseHandler handles a response from the endpoint.
type EndpointResponseHandler interface {
	ProcessResponse(string, *http.Response)
}

// EndpointResponseHandlerFunc is an adapter to allow the use of ordinary
// functions as response handlers. If f is a function with the
// appropriate signature, ResponseHandler(f) is a
// ResponseHandler that calls f.
type EndpointResponseHandlerFunc func(string, *http.Response)

// ProcessResponse calls f(webhookID, resp).
func (f EndpointResponseHandlerFunc) ProcessResponse(webhookID string, resp *http.Response) {
	f(webhookID, resp)
}

// EndpointClient is the client used to POST webhook requests to the local endpoint.
type EndpointClient struct {
	// URL the client sends POST requests to
	URL string

	connect bool

	events map[string]bool

	// Optional configuration parameters
	cfg *EndpointConfig
}

// SupportsEventType takes an event of a webhook and compares it to the internal
// list of supported events
func (c *EndpointClient) SupportsEventType(connect bool, eventType string) bool {
	if connect != c.connect {
		return false
	}

	// Endpoint supports all events, always return true
	if c.events["*"] || c.events[eventType] {
		return true
	}

	return false
}

// Post sends a message to the local endpoint.
func (c *EndpointClient) Post(webhookID string, body string, headers map[string]string) error {
	c.cfg.Log.WithFields(log.Fields{
		"prefix": "proxy.EndpointClient.Post",
	}).Debug("Forwarding event to local endpoint")

	req, err := http.NewRequest(http.MethodPost, c.URL, bytes.NewBuffer([]byte(body)))
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		c.cfg.Log.Errorf("Failed to POST event to local endpoint, error = %v\n", err)
		return err
	}
	defer resp.Body.Close()

	c.cfg.ResponseHandler.ProcessResponse(webhookID, resp)

	return nil
}

//
// Public functions
//

// NewEndpointClient returns a new EndpointClient.
func NewEndpointClient(url string, connect bool, events []string, cfg *EndpointConfig) *EndpointClient {
	if cfg == nil {
		cfg = &EndpointConfig{}
	}
	if cfg.Log == nil {
		cfg.Log = &log.Logger{Out: ioutil.Discard}
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{
			Timeout: defaultTimeout,
		}
	}
	if cfg.ResponseHandler == nil {
		cfg.ResponseHandler = EndpointResponseHandlerFunc(func(string, *http.Response) {})
	}

	return &EndpointClient{
		URL:     url,
		connect: connect,
		events:  convertToMap(events),
		cfg:     cfg,
	}
}

//
// Private constants
//

const (
	defaultTimeout = 30 * time.Second
)

//
// Private functions
//

func convertToMap(events []string) map[string]bool {
	eventsMap := make(map[string]bool)
	for _, event := range events {
		eventsMap[event] = true
	}

	return eventsMap
}
