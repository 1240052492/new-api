package plugins_test

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func callToapiVideoDecode(t *testing.T, plugin *jsplugin.LoadedPlugin, ctx map[string]any) map[string]any {
	t.Helper()
	return callToapiHook(t, plugin, "protocols", []string{"openai_video", "decodeRequest"}, ctx)
}

func callToapiHook(t *testing.T, plugin *jsplugin.LoadedPlugin, hook string, members []string, ctx map[string]any, args ...map[string]any) map[string]any {
	t.Helper()
	params := []any{ctx}
	for _, arg := range args {
		params = append(params, arg)
	}
	value, err := plugin.Engine.CallPath(t.Context(), hook, members, params...)
	require.NoError(t, err)
	encoded, marshalErr := common.Marshal(value)
	require.NoError(t, marshalErr)
	var decoded map[string]any
	require.NoError(t, common.Unmarshal(encoded, &decoded))
	return decoded
}

func callToapiVideoError(t *testing.T, plugin *jsplugin.LoadedPlugin, ctx map[string]any) error {
	t.Helper()
	_, err := plugin.Engine.CallPath(t.Context(), "protocols", []string{"openai_video", "decodeRequest"}, ctx)
	return err
}

func TestToapiVideoDecodeRequest(t *testing.T) {
	plugin, registry := loadToapiPlugin(t)

	t.Run("video models claim the openai_video endpoint", func(t *testing.T) {
		for _, model := range []string{"wan2.6", "wan2.6-flash", "wan3.0-video", "wan3.0-video-prime"} {
			binding, found := registry.Generation().LookupEndpoint("POST", "/v1/videos", model)
			require.True(t, found, model)
			assert.Equal(t, "openai_video", binding.Protocol, model)
		}
		// Image models must not be claimed by the video protocol.
		_, found := registry.Generation().LookupEndpoint("POST", "/v1/videos", "gpt-image-2")
		assert.False(t, found)
	})

	t.Run("wan2.6 text-to-video maps canvas fields", func(t *testing.T) {
		decoded := callToapiVideoDecode(t, plugin, map[string]any{
			"model": "wan2.6", "stream": false,
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model": "wan2.6", "prompt": "一只橘猫在草地上奔跑", "seconds": "5",
				"size": "1280x720", "generate_audio": "false",
			}},
		})
		assert.Equal(t, "submit", decoded["kind"])
		assert.Equal(t, "text_to_video", decoded["action"])
		body := decoded["requestBody"].(map[string]any)
		assert.Equal(t, "wan2.6", body["model"])
		assert.Equal(t, "一只橘猫在草地上奔跑", body["prompt"])
		assert.Equal(t, "16:9", body["aspect_ratio"])
		assert.Equal(t, "720p", body["resolution"])
		assert.Equal(t, float64(5), body["duration"])
		assert.Equal(t, false, body["audio"])
		for _, dropped := range []string{"seconds", "size", "generate_audio", "resolution_name"} {
			assert.NotContains(t, body, dropped)
		}
	})

	t.Run("large landscape sizes pick the 1080p tier", func(t *testing.T) {
		decoded := callToapiVideoDecode(t, plugin, map[string]any{
			"model": "wan2.6", "stream": false,
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model": "wan2.6", "prompt": "cinematic", "size": "1920x1080",
			}},
		})
		body := decoded["requestBody"].(map[string]any)
		assert.Equal(t, "16:9", body["aspect_ratio"])
		assert.Equal(t, "1080p", body["resolution"])
	})

	t.Run("portrait ratio passes through", func(t *testing.T) {
		decoded := callToapiVideoDecode(t, plugin, map[string]any{
			"model": "wan2.6", "stream": false,
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model": "wan2.6", "prompt": "portrait", "aspect_ratio": "9:16", "resolution": "1080p",
			}},
		})
		body := decoded["requestBody"].(map[string]any)
		assert.Equal(t, "9:16", body["aspect_ratio"])
		assert.Equal(t, "1080p", body["resolution"])
	})

	t.Run("wan2.6-flash rejects text-only prompts with the actionable alternative", func(t *testing.T) {
		err := callToapiVideoError(t, plugin, map[string]any{
			"model": "wan2.6-flash", "stream": false,
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model": "wan2.6-flash", "prompt": "一只猫",
			}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not support text-to-video")
		assert.Contains(t, err.Error(), "wan2.6")
	})

	t.Run("wan2.6-flash with an image url routes to image_to_video", func(t *testing.T) {
		decoded := callToapiVideoDecode(t, plugin, map[string]any{
			"model": "wan2.6-flash", "stream": false,
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model": "wan2.6-flash", "prompt": "画面动起来",
				"image_urls": []any{"https://example.com/photo.jpg"},
				"duration": 5, "resolution": "1080p", "audio": true,
			}},
		})
		assert.Equal(t, "image_to_video", decoded["action"])
		body := decoded["requestBody"].(map[string]any)
		assert.Equal(t, []any{"https://example.com/photo.jpg"}, body["image_urls"])
		assert.Equal(t, float64(5), body["duration"])
		assert.NotContains(t, body, "aspect_ratio")
	})

	t.Run("reference videos land in metadata.reference_urls", func(t *testing.T) {
		decoded := callToapiVideoDecode(t, plugin, map[string]any{
			"model": "wan2.6-flash", "stream": false,
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model": "wan2.6-flash", "prompt": "角色在沙发上看电影",
				"metadata": map[string]any{"reference_urls": []any{"https://cdn.example.com/ref.mp4"}},
				"duration": 5,
			}},
		})
		assert.Equal(t, "reference_video", decoded["action"])
		body := decoded["requestBody"].(map[string]any)
		metadata := body["metadata"].(map[string]any)
		assert.Equal(t, []any{"https://cdn.example.com/ref.mp4"}, metadata["reference_urls"])
	})

	t.Run("uploaded files and data urls are rejected", func(t *testing.T) {
		err := callToapiVideoError(t, plugin, map[string]any{
			"model": "wan2.6-flash", "stream": false,
			"body": map[string]any{"kind": "multipart", "fields": map[string]any{}, "files": []any{
				map[string]any{"field": "images", "filename": "cat.png"},
			}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "public http(s) URLs")

		err = callToapiVideoError(t, plugin, map[string]any{
			"model": "wan2.6-flash", "stream": false,
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model": "wan2.6-flash", "image_urls": []any{"data:image/png;base64,AAAA"},
			}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base64")
	})

	t.Run("image urls and reference videos are mutually exclusive", func(t *testing.T) {
		err := callToapiVideoError(t, plugin, map[string]any{
			"model": "wan2.6-flash", "stream": false,
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model":      "wan2.6-flash",
				"image_urls": []any{"https://example.com/a.jpg"},
				"metadata":   map[string]any{"reference_urls": []any{"https://example.com/b.mp4"}},
			}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mutually exclusive")
	})

	t.Run("invalid duration is rejected", func(t *testing.T) {
		err := callToapiVideoError(t, plugin, map[string]any{
			"model": "wan2.6", "stream": false,
			"body": map[string]any{"kind": "json", "value": map[string]any{
				"model": "wan2.6", "prompt": "cat", "seconds": "30",
			}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "seconds")
	})
}

func TestToapiVideoSubmitAndPollContract(t *testing.T) {
	plugin, _ := loadToapiPlugin(t)

	decoded := callToapiVideoDecode(t, plugin, map[string]any{
		"model": "wan2.6", "stream": false,
		"body": map[string]any{"kind": "json", "value": map[string]any{
			"model": "wan2.6", "prompt": "一只橘猫", "seconds": 5, "size": "1280x720", "generate_audio": "false",
		}},
	})

	submit := callToapiHook(t, plugin, "buildSubmitRequest", nil, map[string]any{
		"baseUrl": "https://toapis.cn", "apiKey": "sk-test", "action": decoded["action"],
		"upstreamModel": "wan2.6", "model": "wan2.6", "requestBody": decoded["requestBody"],
	})
	assert.Equal(t, "https://toapis.cn/v1/videos/generations", submit["url"])
	body := submit["body"].(map[string]any)
	assert.Equal(t, "wan2.6", body["model"])

	parse := callToapiHook(t, plugin, "parseSubmitResponse", nil, map[string]any{}, map[string]any{
		"body": map[string]any{
			"id": "tsk_vid_abc", "object": "video", "model": "wan2.6",
			"status": "queued", "progress": 0, "created_at": 1792672516,
		},
	})
	assert.Equal(t, "tsk_vid_abc", parse["taskId"])

	query := callToapiHook(t, plugin, "buildQueryRequest", nil, map[string]any{
		"baseUrl": "https://toapis.cn", "apiKey": "sk-test", "taskId": "tsk_vid_abc",
		"model": "wan2.6", "upstreamModel": "wan2.6",
	})
	assert.Equal(t, "https://toapis.cn/v1/videos/generations/tsk_vid_abc", query["url"])

	// Image tasks keep the image poll endpoint even when the same host polls.
	imageQuery := callToapiHook(t, plugin, "buildQueryRequest", nil, map[string]any{
		"baseUrl": "https://toapis.cn", "apiKey": "sk-test", "taskId": "task_img",
		"model": "gpt-image-2-vip", "upstreamModel": "gpt-image-2-vip",
	})
	assert.Equal(t, "https://toapis.cn/v1/images/generations/task_img", imageQuery["url"])

	usage := callToapiHook(t, plugin, "extractUsage", nil, map[string]any{
		"model": "wan2.6", "upstreamModel": "wan2.6", "requestBody": decoded["requestBody"],
	})
	assert.Equal(t, float64(5), usage["seconds"])
	assert.Equal(t, "720p", usage["resolution"])
	assert.Equal(t, false, usage["audio"])
}

func TestToapiVideoRenderReturnsOpenAIVideoObject(t *testing.T) {
	plugin, _ := loadToapiPlugin(t)

	t.Run("in progress task keeps polling fields", func(t *testing.T) {
		rendered := callToapiHook(t, plugin, "protocols", []string{"openai_video", "render"},
			map[string]any{"model": "wan2.6"},
			map[string]any{
				"task_id": "task_public", "status": "IN_PROGRESS", "progress": "30%", "created_at": 1792672516,
				"properties": map[string]any{"origin_model_name": "wan2.6"},
				"data": map[string]any{
					"id": "tsk_vid_abc", "object": "generation.task", "model": "wan2.6",
					"status": "in_progress", "progress": 30, "created_at": 1792672516,
				},
			})
		assert.Equal(t, "in_progress", rendered["status"])
		assert.Equal(t, float64(30), rendered["progress"])
		assert.Equal(t, "wan2.6", rendered["model"])
		assert.NotContains(t, rendered, "video_url")
	})

	t.Run("completed task exposes video_url from result data", func(t *testing.T) {
		rendered := callToapiHook(t, plugin, "protocols", []string{"openai_video", "render"},
			map[string]any{"model": "wan2.6"},
			map[string]any{
				"task_id": "task_public", "status": "SUCCESS", "created_at": 1792672516, "updated_at": 1792672900,
				"properties": map[string]any{"origin_model_name": "wan2.6"},
				"data": map[string]any{
					"id": "tsk_vid_abc", "status": "completed", "progress": 100,
					"result": map[string]any{"type": "video", "data": []any{
						map[string]any{"url": "https://cdn.toapis.cn/videos/one.mp4", "format": "mp4"},
					}},
				},
			})
		assert.Equal(t, "completed", rendered["status"])
		assert.Equal(t, "https://cdn.toapis.cn/videos/one.mp4", rendered["video_url"])
		assert.Equal(t, float64(1792672900), rendered["completed_at"])
	})

	t.Run("failed task surfaces the upstream error", func(t *testing.T) {
		rendered := callToapiHook(t, plugin, "protocols", []string{"openai_video", "render"},
			map[string]any{"model": "wan2.6"},
			map[string]any{
				"task_id": "task_public", "status": "FAILURE", "created_at": 1,
				"properties": map[string]any{"origin_model_name": "wan2.6"},
				"data": map[string]any{
					"id": "tsk_vid_abc", "status": "failed",
					"error": map[string]any{"code": "content_policy_violation", "message": "内容违规"},
				},
			})
		assert.Equal(t, "failed", rendered["status"])
		errObj := rendered["error"].(map[string]any)
		assert.Equal(t, "content_policy_violation", errObj["code"])
		assert.Equal(t, "内容违规", errObj["message"])
	})
}
