package codextoml

import (
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

func parseDocument(data []byte) (*Document, error) {
	var cfg struct {
		McpServers map[string]map[string]any `toml:"mcp_servers"`
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config.toml: %w", err)
	}

	servers := make(map[string]*Server, len(cfg.McpServers))
	for name, fields := range cfg.McpServers {
		srv, err := serverFromMap(fields)
		if err != nil {
			return nil, fmt.Errorf("parse config.toml: server %q: %w", name, err)
		}
		servers[name] = srv
	}
	return &Document{
		raw:     append([]byte(nil), data...),
		servers: servers,
	}, nil
}

func serverFromMap(fields map[string]any) (*Server, error) {
	srv := &Server{}
	for key, val := range fields {
		var err error
		switch key {
		case "command":
			srv.Command, err = toString(key, val)
		case "args":
			srv.Args, err = toStringSlice(key, val)
		case "env":
			srv.Env, err = toStringMap(key, val)
		case "env_vars":
			srv.EnvVars, err = toStringSlice(key, val)
		case "url":
			srv.URL, err = toString(key, val)
		case "bearer_token_env_var":
			srv.BearerTokenEnvVar, err = toString(key, val)
		case "http_headers":
			srv.HTTPHeaders, err = toStringMap(key, val)
		case "env_http_headers":
			srv.EnvHTTPHeaders, err = toStringMap(key, val)
		default:
			if srv.Other == nil {
				srv.Other = map[string]any{}
			}
			srv.Other[key] = val
		}
		if err != nil {
			return nil, err
		}
	}
	return srv, nil
}

func toString(field string, v any) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("field %q: expected string, got %T", field, v)
	}
	return s, nil
}

func toStringSlice(field string, v any) ([]string, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("field %q: expected array of strings, got %T", field, v)
	}
	if len(arr) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		s, ok := e.(string)
		if !ok {
			return nil, fmt.Errorf("field %q: expected array of strings, got element %T", field, e)
		}
		out = append(out, s)
	}
	return out, nil
}

func toStringMap(field string, v any) (map[string]string, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("field %q: expected table of strings, got %T", field, v)
	}
	if len(m) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(m))
	for k, e := range m {
		s, ok := e.(string)
		if !ok {
			return nil, fmt.Errorf("field %q: key %q: expected string, got %T", field, k, e)
		}
		out[k] = s
	}
	return out, nil
}
