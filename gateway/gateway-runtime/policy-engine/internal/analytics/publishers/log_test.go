/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

// newLogToFile builds a Log publisher that writes to a temp file, returning the
// publisher and a function that reads back what was written.
func newLogToFile(t *testing.T, cfg *config.TrafficLoggingConfig) (*Log, func() string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out.log")
	f, err := os.Create(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	l := NewLog(cfg)
	l.out = f
	return l, func() string {
		require.NoError(t, f.Sync())
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		return string(data)
	}
}

func TestNewLog_NilConfig(t *testing.T) {
	l := NewLog(nil)
	require.NotNil(t, l)
	assert.Empty(t, l.maskedHeaders)
}

func TestLog_Publish_WritesJSONLine(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"x-foo":"bar"}`

	l.Publish(event)

	out := read()
	// Single line (one trailing newline).
	assert.Equal(t, 1, strings.Count(strings.TrimRight(out, "\n"), "\n")+1)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	api := decoded["api"].(map[string]interface{})
	assert.Equal(t, "test-api", api["apiName"])
	props := decoded["properties"].(map[string]interface{})
	assert.Equal(t, `{"x-foo":"bar"}`, props["requestHeaders"])
}

func TestLog_Publish_MasksHeaders(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{MaskedHeaders: []string{"Authorization"}})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = `{"Authorization":"Bearer secret","x-foo":"bar"}`
	event.Properties["responseHeaders"] = `{"authorization":"Bearer secret2"}`

	l.Publish(event)

	out := read()
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	props := decoded["properties"].(map[string]interface{})

	var reqH map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(props["requestHeaders"].(string)), &reqH))
	assert.Equal(t, "****", reqH["Authorization"]) // masked
	assert.Equal(t, "bar", reqH["x-foo"])          // untouched

	var resH map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(props["responseHeaders"].(string)), &resH))
	assert.Equal(t, "****", resH["authorization"]) // case-insensitive match
}

func TestLog_Publish_DoesNotMutateSharedEvent(t *testing.T) {
	l, _ := newLogToFile(t, &config.TrafficLoggingConfig{MaskedHeaders: []string{"authorization"}})
	event := createBaseEvent()
	original := `{"authorization":"Bearer secret"}`
	event.Properties["requestHeaders"] = original

	l.Publish(event)

	// The shared event (read by other publishers) must be untouched.
	assert.Equal(t, original, event.Properties["requestHeaders"])
}

func TestLog_Publish_NilEvent(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{})
	assert.NotPanics(t, func() { l.Publish(nil) })
	assert.Empty(t, read())
}

func TestLog_Publish_InvalidHeaderJSONLeftAsIs(t *testing.T) {
	l, read := newLogToFile(t, &config.TrafficLoggingConfig{MaskedHeaders: []string{"authorization"}})
	event := createBaseEvent()
	event.Properties["requestHeaders"] = "not-json"

	l.Publish(event)

	out := read()
	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	props := decoded["properties"].(map[string]interface{})
	assert.Equal(t, "not-json", props["requestHeaders"])
}
