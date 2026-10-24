package plugins_test

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadToapiPlugin(t *testing.T) (*jsplugin.LoadedPlugin, *jsplugin.Registry) {
	t.Helper()
	source, err := builtinplugins.Source("toapi")
	require.NoError(t, err)
	registry := jsplugin.NewRegistry()
	plugin, err := registry.RegisterFactory(source, jsplugin.Options{Key: "toapi"})
	require.NoError(t, err)
	return plugin, registry
}

func TestToapiPluginClaimsImageEndpoints(t *testing.T) {
	plugin, registry := loadToapiPlugin(t)
	require.NotEmpty(t, plugin.Meta.Models)

	for _, model := range plugin.Meta.Models {
		// Video models (wan/grok-imagine-video) are claimed by the
		// openai_video protocol, not the image endpoints.
		if strings.Contains(model, "video") || strings.HasPrefix(model, "wan") {
			continue
		}
		for _, path := range []string{"/v1/images/generations", "/v1/images/edits"} {
			binding, found := registry.Generation().LookupEndpoint("POST", path, model)
			require.True(t, found, model+" "+path)
			assert.Same(t, plugin, binding.Plugin, model+" "+path)
			assert.Equal(t, "openai_image", binding.Protocol, model+" "+path)
		}
	}

	// Models claimed by other plugins must stay untouched.
	for _, model := range []string{"qwen-image-3.0", "qwen-image-3.0-pro"} {
		_, found := registry.Generation().LookupEndpoint("POST", "/v1/images/generations", model)
		assert.False(t, found, model)
	}
}

func callToapiDecode(t *testing.T, plugin *jsplugin.LoadedPlugin, operation string, ctx map[string]any) map[string]any {
	t.Helper()
	if ctx["operation"] == nil {
		ctx["operation"] = operation
	}
	value, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_image", "decodeRequest"}, ctx)
	require.NoError(t, err)
	encoded, marshalErr := common.Marshal(value)
	require.NoError(t, marshalErr)
	var decoded map[string]any
	require.NoError(t, common.Unmarshal(encoded, &decoded))
	return decoded
}

func TestToapiDecodeRequestMapsPixelSizes(t *testing.T) {
	plugin, _ := loadToapiPlugin(t)

	t.Run("canvas style pixel request becomes ratio and tier", func(t *testing.T) {
		decoded := callToapiDecode(t, plugin, "generate", map[string]any{
			"model": "gpt-image-2-vip", "stream": false,
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model": "gpt-image-2-vip", "prompt": "a candid photo", "n": 1,
				"size": "2048x2048", "background": "transparent", "output_format": "png",
			}},
		})
		assert.Equal(t, "submit", decoded["kind"])
		assert.Equal(t, "text_to_image", decoded["action"])
		body, ok := decoded["requestBody"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "gpt-image-2-vip", body["model"])
		assert.Equal(t, "a candid photo", body["prompt"])
		assert.Equal(t, "1:1", body["size"])
		assert.Equal(t, "2k", body["resolution"])
		assert.Equal(t, "transparent", body["background"])
		for _, dropped := range []string{"output_format", "quality", "response_format"} {
			assert.NotContains(t, body, dropped)
		}
	})

	t.Run("landscape pixel size maps onto the nearest documented ratio", func(t *testing.T) {
		decoded := callToapiDecode(t, plugin, "generate", map[string]any{
			"model": "gpt-image-2", "stream": false,
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model": "gpt-image-2", "prompt": "poster", "size": "2048x1152",
			}},
		})
		body := decoded["requestBody"].(map[string]any)
		assert.Equal(t, "16:9", body["size"])
		assert.Equal(t, "2k", body["resolution"])
	})

	t.Run("ratio sizes pass through with normalized resolution", func(t *testing.T) {
		decoded := callToapiDecode(t, plugin, "generate", map[string]any{
			"model": "gpt-image-2", "stream": false,
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model": "gpt-image-2", "prompt": "poster", "size": "16:9", "resolution": "4K",
			}},
		})
		body := decoded["requestBody"].(map[string]any)
		assert.Equal(t, "16:9", body["size"])
		assert.Equal(t, "4k", body["resolution"])
	})

	t.Run("url references keep the edit operation usable", func(t *testing.T) {
		decoded := callToapiDecode(t, plugin, "edit", map[string]any{
			"model": "gpt-image-2", "stream": false,
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model": "gpt-image-2", "prompt": "make it night", "n": 1,
				"reference_images": []any{"https://example.com/in.png"},
			}},
		})
		assert.Equal(t, "image_to_image", decoded["action"])
		body := decoded["requestBody"].(map[string]any)
		assert.Equal(t, []any{"https://example.com/in.png"}, body["reference_images"])
	})

	t.Run("multipart edits are rejected with an actionable reason", func(t *testing.T) {
		_, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_image", "decodeRequest"}, map[string]any{
			"model": "gpt-image-2", "operation": "edit", "stream": false,
			"body": map[string]any{"kind": "multipart", "fields": map[string]any{}, "files": []any{}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "JSON body required")
	})

	t.Run("missing prompt is rejected", func(t *testing.T) {
		_, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_image", "decodeRequest"}, map[string]any{
			"model": "gpt-image-2", "operation": "generate", "stream": false,
			"body": map[string]any{"kind": "json", "value": map[string]any{"model": "gpt-image-2"}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "prompt is required")
	})
}

func decodeToapiIntent(t *testing.T, plugin *jsplugin.LoadedPlugin, value map[string]any) map[string]any {
	t.Helper()
	decoded := callToapiDecode(t, plugin, "generate", map[string]any{
		"model": value["model"], "stream": false,
		"body": map[string]any{"kind": "json", "value": value},
	})
	return decoded
}

func TestToapiSubmitAndPollContract(t *testing.T) {
	plugin, _ := loadToapiPlugin(t)
	decoded := decodeToapiIntent(t, plugin, map[string]any{
		"model": "gpt-image-2-vip", "prompt": "a candid photo", "n": 1, "size": "2048x2048",
	})

	submitRaw, err := plugin.Engine.CallPath(t.Context(), "buildSubmitRequest", nil, map[string]any{
		"baseUrl": "https://toapis.cn", "apiKey": "sk-test", "action": decoded["action"],
		"upstreamModel": "gpt-image-2-vip", "requestBody": decoded["requestBody"],
	})
	require.NoError(t, err)
	encoded, marshalErr := common.Marshal(submitRaw)
	require.NoError(t, marshalErr)
	var submit map[string]any
	require.NoError(t, common.Unmarshal(encoded, &submit))
	assert.Equal(t, "https://toapis.cn/v1/images/generations", submit["url"])
	assert.Equal(t, "POST", submit["method"])
	headers := submit["headers"].(map[string]any)
	assert.Equal(t, "Bearer sk-test", headers["Authorization"])
	body := submit["body"].(map[string]any)
	assert.Equal(t, "gpt-image-2-vip", body["model"], "upstream model mapping must win")

	parseRaw, err := plugin.Engine.CallPath(t.Context(), "parseSubmitResponse", nil, map[string]any{}, map[string]any{
		"body": map[string]any{
			"id": "task_img_abc123", "object": "generation.task", "model": "gpt-image-2-vip",
			"status": "queued", "progress": 0, "created_at": 1703884800, "metadata": map[string]any{},
		},
	})
	require.NoError(t, err)
	encoded, marshalErr = common.Marshal(parseRaw)
	require.NoError(t, marshalErr)
	var parsed map[string]any
	require.NoError(t, common.Unmarshal(encoded, &parsed))
	assert.Equal(t, "task_img_abc123", parsed["taskId"])

	queryRaw, err := plugin.Engine.CallPath(t.Context(), "buildQueryRequest", nil, map[string]any{
		"baseUrl": "https://toapis.cn", "apiKey": "sk-test", "taskId": "task_img_abc123",
	})
	require.NoError(t, err)
	encoded, marshalErr = common.Marshal(queryRaw)
	require.NoError(t, marshalErr)
	var query map[string]any
	require.NoError(t, common.Unmarshal(encoded, &query))
	assert.Equal(t, "https://toapis.cn/v1/images/generations/task_img_abc123", query["url"])
	assert.Equal(t, "GET", query["method"])

	statuses := []struct {
		name     string
		body     map[string]any
		expected string
		progress string
		reason   string
	}{
		{"queued", map[string]any{"status": "queued", "progress": 0}, "QUEUED", "0%", ""},
		{"in_progress", map[string]any{"status": "in_progress", "progress": 50}, "IN_PROGRESS", "50%", ""},
		{"failed", map[string]any{"status": "failed", "error": map[string]any{"code": "content_policy_violation", "message": "内容违规"}}, "FAILURE", "100%", "内容违规"},
	}
	for _, tc := range statuses {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := plugin.Engine.CallPath(t.Context(), "parseTaskResult", nil, map[string]any{}, tc.body)
			require.NoError(t, err)
			encoded, marshalErr := common.Marshal(raw)
			require.NoError(t, marshalErr)
			var result map[string]any
			require.NoError(t, common.Unmarshal(encoded, &result))
			assert.Equal(t, tc.expected, result["status"])
			assert.Equal(t, tc.progress, result["progress"])
			if tc.reason != "" {
				assert.Equal(t, tc.reason, result["reason"])
			}
		})
	}

	t.Run("completed result exposes the first image url", func(t *testing.T) {
		raw, err := plugin.Engine.CallPath(t.Context(), "parseTaskResult", nil, map[string]any{}, map[string]any{
			"id": "task_img_abc123", "status": "completed", "progress": 100,
			"result": map[string]any{"type": "image", "data": []any{
				map[string]any{"url": "https://cdn.toapis.cn/images/one.png"},
				map[string]any{"url": "https://cdn.toapis.cn/images/two.png"},
			}},
			"usage": map[string]any{"output_tokens": 1200, "total_tokens": 1300},
		})
		require.NoError(t, err)
		encoded, marshalErr := common.Marshal(raw)
		require.NoError(t, marshalErr)
		var result map[string]any
		require.NoError(t, common.Unmarshal(encoded, &result))
		assert.Equal(t, "SUCCESS", result["status"])
		assert.Equal(t, "https://cdn.toapis.cn/images/one.png", result["url"])
		assert.Equal(t, "100%", result["progress"])
	})

	t.Run("unrecognized status is reported as unknown", func(t *testing.T) {
		raw, err := plugin.Engine.CallPath(t.Context(), "parseTaskResult", nil, map[string]any{}, map[string]any{"status": "mystery"})
		require.NoError(t, err)
		encoded, marshalErr := common.Marshal(raw)
		require.NoError(t, marshalErr)
		var result map[string]any
		require.NoError(t, common.Unmarshal(encoded, &result))
		assert.Equal(t, "UNKNOWN", result["status"])
	})
}

func TestToapiRenderReturnsImageEntries(t *testing.T) {
	plugin, _ := loadToapiPlugin(t)
	value, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_image", "render"},
		map[string]any{"model": "gpt-image-2-vip"},
		map[string]any{
			"task_id": "task_public", "status": "SUCCESS", "created_at": 1768381010,
			"data": map[string]any{
				"id": "task_img_abc123", "object": "generation.task", "status": "completed", "progress": 100,
				"result": map[string]any{"type": "image", "data": []any{
					map[string]any{"url": "https://cdn.toapis.cn/images/one.png"},
					map[string]any{"url": "https://cdn.toapis.cn/images/two.png"},
				}},
			},
		})
	require.NoError(t, err)
	encoded, marshalErr := common.Marshal(value)
	require.NoError(t, marshalErr)
	var rendered map[string]any
	require.NoError(t, common.Unmarshal(encoded, &rendered))
	assert.Equal(t, float64(1768381010), rendered["created"])
	data, ok := rendered["data"].([]any)
	require.True(t, ok)
	require.Len(t, data, 2)
	assert.Equal(t, "https://cdn.toapis.cn/images/one.png", data[0].(map[string]any)["url"])
	assert.Equal(t, "https://cdn.toapis.cn/images/two.png", data[1].(map[string]any)["url"])

	t.Run("missing images fail loudly", func(t *testing.T) {
		_, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_image", "render"},
			map[string]any{"model": "gpt-image-2-vip"},
			map[string]any{"task_id": "task_public", "status": "SUCCESS", "created_at": 1, "data": map[string]any{"status": "completed"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "image result is unavailable")
	})
}
