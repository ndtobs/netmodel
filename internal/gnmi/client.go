package gnmi

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// Client wraps a gNMI client connection
type Client struct {
	conn     *grpc.ClientConn
	client   gnmi.GNMIClient
	target   string
	username string
	password string
}

// Config holds connection configuration
type Config struct {
	Address  string
	Username string
	Password string
	Insecure bool
	Timeout  time.Duration
}

// NewClient creates a new gNMI client
func NewClient(cfg Config) (*Client, error) {
	var opts []grpc.DialOption

	if cfg.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, cfg.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	return &Client{
		conn:     conn,
		client:   gnmi.NewGNMIClient(conn),
		target:   cfg.Address,
		username: cfg.Username,
		password: cfg.Password,
	}, nil
}

// Close closes the client connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// GetJSON performs a gNMI Get request and returns parsed JSON
func (c *Client) GetJSON(ctx context.Context, path string) (map[string]interface{}, error) {
	gnmiPath, err := ParsePath(path)
	if err != nil {
		return nil, fmt.Errorf("parse path: %w", err)
	}

	req := &gnmi.GetRequest{
		Path:     []*gnmi.Path{gnmiPath},
		Encoding: gnmi.Encoding_JSON_IETF,
	}

	if c.username != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "username", c.username, "password", c.password)
	}

	resp, err := c.client.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get: %w", err)
	}

	if len(resp.Notification) == 0 || len(resp.Notification[0].Update) == 0 {
		return nil, nil
	}

	update := resp.Notification[0].Update[0]
	jsonData := extractJSONValue(update.Val)
	if jsonData == nil {
		return nil, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		// Maybe it's a different type, wrap it
		var anyResult interface{}
		if err := json.Unmarshal(jsonData, &anyResult); err != nil {
			return nil, fmt.Errorf("unmarshal JSON: %w", err)
		}
		return map[string]interface{}{"value": anyResult}, nil
	}

	return result, nil
}

// GetValue performs a gNMI Get request for a single value
func (c *Client) GetValue(ctx context.Context, path string) (string, bool, error) {
	gnmiPath, err := ParsePath(path)
	if err != nil {
		return "", false, fmt.Errorf("parse path: %w", err)
	}

	req := &gnmi.GetRequest{
		Path:     []*gnmi.Path{gnmiPath},
		Encoding: gnmi.Encoding_JSON_IETF,
	}

	if c.username != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "username", c.username, "password", c.password)
	}

	resp, err := c.client.Get(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "not found") {
			return "", false, nil
		}
		return "", false, fmt.Errorf("get: %w", err)
	}

	if len(resp.Notification) == 0 || len(resp.Notification[0].Update) == 0 {
		return "", false, nil
	}

	update := resp.Notification[0].Update[0]
	value := extractStringValue(update.Val)

	return value, true, nil
}

// ParsePath converts a string path to a gNMI Path
func ParsePath(path string) (*gnmi.Path, error) {
	path = strings.TrimPrefix(path, "/")

	var elems []*gnmi.PathElem
	for _, segment := range splitPath(path) {
		elem, err := parsePathElem(segment)
		if err != nil {
			return nil, err
		}
		elems = append(elems, elem)
	}

	return &gnmi.Path{Elem: elems}, nil
}

func splitPath(path string) []string {
	var segments []string
	var current strings.Builder
	depth := 0

	for _, r := range path {
		switch r {
		case '[':
			depth++
			current.WriteRune(r)
		case ']':
			depth--
			current.WriteRune(r)
		case '/':
			if depth == 0 {
				if current.Len() > 0 {
					segments = append(segments, current.String())
					current.Reset()
				}
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		segments = append(segments, current.String())
	}

	return segments
}

func parsePathElem(segment string) (*gnmi.PathElem, error) {
	elem := &gnmi.PathElem{
		Key: make(map[string]string),
	}

	bracketStart := strings.Index(segment, "[")
	if bracketStart == -1 {
		elem.Name = segment
		return elem, nil
	}

	elem.Name = segment[:bracketStart]

	keysPart := segment[bracketStart:]
	for len(keysPart) > 0 {
		if keysPart[0] != '[' {
			break
		}
		end := strings.Index(keysPart, "]")
		if end == -1 {
			return nil, fmt.Errorf("unclosed bracket in path segment: %s", segment)
		}

		kv := keysPart[1:end]
		eqIdx := strings.Index(kv, "=")
		if eqIdx == -1 {
			return nil, fmt.Errorf("invalid key-value pair: %s", kv)
		}

		key := kv[:eqIdx]
		value := kv[eqIdx+1:]
		elem.Key[key] = value

		keysPart = keysPart[end+1:]
	}

	return elem, nil
}

func extractJSONValue(val *gnmi.TypedValue) []byte {
	if val == nil {
		return nil
	}

	switch v := val.Value.(type) {
	case *gnmi.TypedValue_JsonVal:
		return v.JsonVal
	case *gnmi.TypedValue_JsonIetfVal:
		return v.JsonIetfVal
	default:
		// For non-JSON values, convert to JSON
		str := extractStringValue(val)
		data, _ := json.Marshal(str)
		return data
	}
}

func extractStringValue(val *gnmi.TypedValue) string {
	if val == nil {
		return ""
	}

	switch v := val.Value.(type) {
	case *gnmi.TypedValue_StringVal:
		return v.StringVal
	case *gnmi.TypedValue_IntVal:
		return fmt.Sprintf("%d", v.IntVal)
	case *gnmi.TypedValue_UintVal:
		return fmt.Sprintf("%d", v.UintVal)
	case *gnmi.TypedValue_BoolVal:
		return fmt.Sprintf("%t", v.BoolVal)
	case *gnmi.TypedValue_FloatVal:
		return fmt.Sprintf("%f", v.FloatVal)
	case *gnmi.TypedValue_JsonVal:
		return string(v.JsonVal)
	case *gnmi.TypedValue_JsonIetfVal:
		return string(v.JsonIetfVal)
	case *gnmi.TypedValue_AsciiVal:
		return v.AsciiVal
	default:
		return fmt.Sprintf("%v", val.Value)
	}
}
