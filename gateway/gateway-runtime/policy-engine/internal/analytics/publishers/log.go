/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package publishers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"strings"
	"sync"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

// maskedHeaderValue is the placeholder written in place of a masked header value.
const maskedHeaderValue = "****"

// Log is an analytics publisher that writes each enriched analytics event to
// stdout as a single JSON line. It is intended for log-scraping pipelines
// (Fluent Bit, Loki, ELK, etc.) and as a lightweight alternative to a SaaS
// analytics backend. The event already carries the rich metadata, headers and
// (when send_request_body/send_response_body are enabled) payloads attached by
// the analytics engine, so this publisher only serializes it.
type Log struct {
	pretty bool
	// maskedHeaders holds lower-cased header names whose values are redacted in
	// the requestHeaders/responseHeaders properties before logging.
	maskedHeaders map[string]bool
	// mu serializes writes to stdout so concurrent ALS streams do not interleave.
	mu sync.Mutex
	// out is the destination writer; defaults to os.Stdout (overridable in tests).
	out *os.File
}

// NewLog creates a new stdout log publisher.
func NewLog(logCfg *config.LogPublisherConfig) *Log {
	if logCfg == nil {
		logCfg = &config.LogPublisherConfig{}
	}

	masked := make(map[string]bool, len(logCfg.MaskedHeaders))
	for _, h := range logCfg.MaskedHeaders {
		h = strings.ToLower(strings.TrimSpace(h))
		if h != "" {
			masked[h] = true
		}
	}

	return &Log{
		pretty:        logCfg.Pretty,
		maskedHeaders: masked,
		out:           os.Stdout,
	}
}

// Publish writes the event to stdout as JSON.
func (l *Log) Publish(event *dto.Event) {
	if event == nil {
		return
	}

	out := l.applyMasking(event)

	var (
		data []byte
		err  error
	)
	if l.pretty {
		data, err = json.MarshalIndent(out, "", "  ")
	} else {
		data, err = json.Marshal(out)
	}
	if err != nil {
		slog.Error("Failed to marshal analytics event for log publisher", "error", err)
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := fmt.Fprintln(l.out, string(data)); err != nil {
		slog.Error("Failed to write analytics event to stdout", "error", err)
	}
}

// applyMasking returns the event to serialize. When masked headers are
// configured, it returns a shallow copy with a cloned Properties map whose
// requestHeaders/responseHeaders values have the configured headers redacted,
// so the shared event observed by other publishers is left untouched.
func (l *Log) applyMasking(event *dto.Event) *dto.Event {
	if len(l.maskedHeaders) == 0 || event.Properties == nil {
		return event
	}

	props := make(map[string]interface{}, len(event.Properties))
	maps.Copy(props, event.Properties)
	for _, key := range []string{"requestHeaders", "responseHeaders"} {
		if raw, ok := props[key].(string); ok {
			props[key] = l.maskHeaders(raw)
		}
	}

	cp := *event
	cp.Properties = props
	return &cp
}

// maskHeaders parses a JSON header map and redacts the values of any configured
// masked headers (case-insensitive). The raw string is returned unchanged if it
// is empty or not valid JSON.
func (l *Log) maskHeaders(raw string) string {
	if raw == "" {
		return raw
	}
	var headers map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return raw
	}
	for name := range headers {
		if l.maskedHeaders[strings.ToLower(name)] {
			headers[name] = maskedHeaderValue
		}
	}
	masked, err := json.Marshal(headers)
	if err != nil {
		return raw
	}
	return string(masked)
}
