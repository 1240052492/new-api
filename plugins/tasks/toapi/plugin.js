// Wan video models share one submit/poll surface: POST/GET
// /v1/videos/generations with a generation.task body. wan2.6 supports
// text-to-video; -flash variants require image_urls or reference videos.
const VIDEO_MODELS = ["wan2.6", "wan2.6-flash", "wan3.0-video", "wan3.0-video-prime", "grok-imagine-video-official"];

const IMAGE_MODELS = [
  "gpt-image-2",
  "gpt-image-2-vip",
  "gpt-image-2-official",
  "gpt-image-2.5-sunburst",
  "gpt-image-2.5-sunburst-vip",
  "gpt-image-2.5-sunburst-official",
  "gpt-image-2.5-flare",
  "gpt-image-2.5-flare-vip",
  "gpt-image-2.5-flare-official",
  "gemini-nano-banana-2.1",
  "gemini-nano-banana-2.1-vip",
  "gemini-nano-banana-2.1-official",
  "gemini-3-pro-image-preview",
  "gemini-3-pro-image-preview-vip",
  "gemini-3-pro-image-official",
  "gemini-3.1-flash-image-preview",
  "gemini-3.1-flash-image-preview-vip",
  "gemini-3.1-flash-image-official",
  "gemini-3.1-flash-lite-image-official",
  "gemini-2.5-flash-image-preview",
  "gemini-2.5-flash-image-official",
  "nano_banana",
  "nano_banana_2",
  "nano_banana_pro",
  "flux-2-flex",
  "flux-2-pro",
  "flux-kontext-max",
  "flux-kontext-pro",
  "doubao-seedream-4-0",
  "doubao-seedream-4-5",
  "doubao-seedream-5-0",
  "doubao-seedream-5-0-pro",
  "vidu-image",
];

export const meta = {
  apiVersion: 1,
  key: "toapi",
  name: "ToAPIs",
  icon: "ToAPIs",
  description: {
    en: "ToAPIs unified async generation: images (gpt-image, Gemini image, FLUX, Seedream) over the synchronous OpenAI Images API, and Wan video over the async video task API",
    zh: "ToAPIs 统一异步生成：图像（gpt-image、Gemini 图像、FLUX、Seedream 等）保持同步 OpenAI Images 协议，万相视频走异步视频任务协议",
  },
  version: "1.1.2",
  author: { name: "QuantumNous" },
  // Bound explicitly through a New API channel's task_plugin_key /
  // task_extend_plugin_keys setting (e.g. the https://toapis.cn channel).
  models: IMAGE_MODELS.concat(VIDEO_MODELS),
  fetchMode: "per_task",
  upstreams: ["vendor", "new_api"],
  // Per-model-group usage schemas. The image schema must not declare the
  // video `resolution` enum because the image decode emits resolution tiers
  // ("1k"/"2k"/"4k"); a shared schema rejected every image request with
  // "plugin usage enum is not an allowed value".
  usageProfiles: [
    {
      models: IMAGE_MODELS,
      schema: {
        image_count: {
          type: "number",
          unit: "count",
          description: { en: "Requested output image count", zh: "请求的输出图片数量" },
        },
      },
    },
    {
      models: VIDEO_MODELS,
      schema: {
        seconds: {
          type: "number",
          unit: "second",
          description: { en: "Requested video duration", zh: "请求的视频时长" },
        },
        resolution: {
          enum: ["720p", "1080p"],
          enumLabels: { "720p": { en: "720p", zh: "720p" }, "1080p": { en: "1080p", zh: "1080p" } },
          description: { en: "Output video resolution", zh: "输出视频分辨率" },
        },
        audio: {
          type: "boolean",
          description: { en: "Whether the video carries audio", zh: "视频是否带音频" },
        },
      },
    },
  ],
  protocols: [{ name: "openai_image", models: IMAGE_MODELS }, { name: "openai_video", models: VIDEO_MODELS }],
};

// ToAPIs runs every image model as an async task: POST /v1/images/generations
// answers 200 with a generation.task object, and the images are fetched later
// from GET /v1/images/generations/{task_id}. The host image protocol bridge
// submits the durable task, polls it inside the client request, and renders
// the OpenAI Images response once the task is terminal.

const MAX_IMAGE_COUNT = 15;
const MAX_REFERENCE_IMAGES = 14;
const SIZE_RATIOS = [
  "1:1", "3:2", "2:3", "4:3", "3:4", "5:4", "4:5",
  "16:9", "9:16", "2:1", "1:2", "21:9", "9:21",
  "1:4", "4:1", "1:8", "8:1",
];
const RESOLUTIONS = ["1k", "2k", "4k"];
// Wan video models share one submit/poll surface: POST/GET
// /v1/videos/generations with a generation.task body. wan2.6 supports
// text-to-video; -flash variants require image_urls or reference videos.
const VIDEO_RATIOS = ["16:9", "9:16", "1:1", "4:3", "3:4", "3:2", "2:3"];
const VIDEO_RESOLUTIONS = ["720p", "1080p"];
const MAX_VIDEO_DURATION = 15;

function isVideoModel(value) {
  const name = trimmed(typeof value === "object" && value !== null ? value.model || value.upstreamModel : value).toLowerCase();
  return name.startsWith("wan2.6") || name.startsWith("wan3.0") || name.startsWith("grok-imagine-video");
}

function nearestVideoRatio(value) {
  const target = ratioValue(value);
  if (target === null) return "";
  let best = "";
  let bestDelta = Infinity;
  for (const candidate of VIDEO_RATIOS) {
    const delta = Math.abs(ratioValue(candidate) - target) / target;
    if (delta < bestDelta) {
      best = candidate;
      bestDelta = delta;
    }
  }
  return best;
}

// Wan video bills two resolution tiers; keep the longer edge above 1080
// pixels on the 1080p tier, everything else on 720p.
function videoResolutionTier(width, height) {
  const longer = Math.max(width, height);
  const shorter = Math.min(width, height);
  return shorter >= 1000 || longer >= 1600 ? "1080p" : "720p";
}

function normalizeVideoResolution(value) {
  const raw = trimmed(value).toLowerCase().replace("480p", "720p");
  return VIDEO_RESOLUTIONS.includes(raw) ? raw : "";
}

function normalizeDuration(value) {
  const duration = Number(value);
  if (!Number.isFinite(duration) || duration < 1 || duration > MAX_VIDEO_DURATION) return 0;
  return Math.round(duration);
}

function normalizeBool(value) {
  if (value === undefined || value === null || value === "") return undefined;
  if (typeof value === "boolean") return value;
  if (value === "true") return true;
  if (value === "false") return false;
  return undefined;
}

// Accepts an array of URLs, a single URL string, or the multipart spelling of
// repeated fields; uploaded files are rejected because ToAPIs only accepts
// public URLs for references.
function referenceUrlList(values, field) {
  const urls = [];
  const entries = Array.isArray(values) ? values : values === undefined || values === null || values === "" ? [] : [values];
  for (const item of entries) {
    const url = trimmed(item && typeof item === "object" ? item.url : item);
    if (!url) continue;
    if (/^data:/i.test(url) || (item && typeof item === "object" && item.storageKey)) {
      throw new Error(field + " must be public http(s) URLs; ToAPIs does not accept uploaded files or base64 data");
    }
    if (!urls.includes(url)) urls.push(url);
  }
  return urls;
}

function trimmed(value) {
  return String(value || "").trim();
}

function ratioValue(value) {
  const parts = value.split(":");
  if (parts.length !== 2) return null;
  const width = Number(parts[0]);
  const height = Number(parts[1]);
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return null;
  return width / height;
}

// TOAPIs accepts ratio spellings only ("1:1", "16:9", ...); pick the closest
// declared ratio for whatever the client sent.
function nearestRatio(value) {
  const target = ratioValue(value);
  if (target === null) return null;
  let best = null;
  let bestDelta = Infinity;
  for (const candidate of SIZE_RATIOS) {
    const base = ratioValue(candidate);
    const delta = Math.abs(base - target) / target;
    if (delta < bestDelta) {
      best = candidate;
      bestDelta = delta;
    }
  }
  return best;
}

// Documented pixel tiers: 1k tops out at a 1536 long edge, 2k at 2688.
function resolutionTier(width, height) {
  const longest = Math.max(width, height);
  if (longest <= 1536) return "1k";
  if (longest <= 2688) return "2k";
  return "4k";
}

function normalizeResolution(value) {
  const raw = trimmed(value).toLowerCase();
  return RESOLUTIONS.includes(raw) ? raw : "";
}

function jsonRequest(ctx) {
  if (!ctx.body || ctx.body.kind !== "json") {
    throw new Error("JSON body required; ToAPIs image models only accept reference images as public URLs");
  }
  const req = ctx.body.value;
  if (!req || typeof req !== "object" || Array.isArray(req)) throw new Error("request body must be an object");
  return req;
}

function referenceUrls(req) {
  const references = [];
  for (const field of ["reference_images", "image_urls"]) {
    if (req[field] === undefined || req[field] === null) continue;
    if (!Array.isArray(req[field])) throw new Error(field + " must be an array of image URLs");
    for (const item of req[field]) {
      const url = trimmed(item);
      if (url && !references.includes(url)) references.push(url);
    }
  }
  if (references.length > MAX_REFERENCE_IMAGES) {
    throw new Error("at most " + MAX_REFERENCE_IMAGES + " reference images are supported");
  }
  return references;
}

export const protocols = {
  openai_image: {
    decodeRequest: function (ctx) {
      const req = jsonRequest(ctx);
      const model = trimmed(ctx.model);
      if (!model) throw new Error("model is required");
      const prompt = trimmed(req.prompt);
      if (!prompt) throw new Error("prompt is required");
      if (req.stream) {
        throw new Error("stream is not supported; the complete image response is returned once the task finishes");
      }
      if (req.response_format !== undefined && req.response_format !== "url" && req.response_format !== "b64_json") {
        throw new Error("response_format must be url or b64_json");
      }
      let n = 1;
      if (req.n !== undefined && req.n !== null) {
        n = Number(req.n);
        if (!Number.isInteger(n) || n < 1 || n > MAX_IMAGE_COUNT) {
          throw new Error("n must be an integer between 1 and " + MAX_IMAGE_COUNT);
        }
      }
      const references = referenceUrls(req);
      if (ctx.operation === "edit" && references.length === 0) {
        throw new Error("image edits require reference_images with public image URLs; ToAPIs does not accept uploaded image files");
      }

      const body = { model: model, prompt: prompt, n: n };
      if (references.length) body.reference_images = references;
      // Only "transparent" exists upstream; every other value means the
      // default opaque background and must be omitted entirely.
      if (trimmed(req.background).toLowerCase() === "transparent") body.background = "transparent";

      const size = trimmed(req.size).toLowerCase();
      if (size && size !== "auto") {
        const dimensions = /^(\d+)x(\d+)$/.exec(size);
        if (dimensions) {
          const width = Number(dimensions[1]);
          const height = Number(dimensions[2]);
          const ratio = nearestRatio(width + ":" + height);
          if (!ratio) throw new Error("size is not supported");
          body.size = ratio;
          body.resolution = resolutionTier(width, height);
        } else {
          const ratio = nearestRatio(size);
          if (!ratio) throw new Error("size must be a ratio such as 1:1 or pixel dimensions such as 2048x2048");
          body.size = ratio;
          body.resolution = normalizeResolution(req.resolution) || "1k";
        }
      } else {
        const resolution = normalizeResolution(req.resolution);
        if (resolution) body.resolution = resolution;
      }

      return {
        kind: "submit",
        model: model,
        action: references.length ? "image_to_image" : "text_to_image",
        requestBody: body,
      };
    },
    render: function (ctx, task) {
      const urls = resultImageUrls(task);
      if (!urls.length) throw new Error("image result is unavailable");
      return {
        created: task.created_at,
        data: urls.map(function (url) {
          return { url: url };
        }),
      };
    },
  },
  openai_video: {
    decodeRequest: function (ctx) {
      let req;
      if (ctx.body && ctx.body.kind === "json") {
        req = ctx.body.value;
        if (!req || typeof req !== "object" || Array.isArray(req)) throw new Error("request body must be an object");
      } else if (ctx.body && ctx.body.kind === "multipart") {
        if ((ctx.body.files || []).length) {
          throw new Error("uploaded files are not supported; reference images and videos must be public http(s) URLs");
        }
        req = {};
        const fields = ctx.body.fields || {};
        for (const name of Object.keys(fields)) {
          req[name] = fields[name].length === 1 ? fields[name][0] : fields[name];
        }
      } else throw new Error("JSON or multipart body required");

      const model = trimmed(ctx.model);
      if (!model) throw new Error("model is required");
      const video = isVideoModel(model);

      const prompt = trimmed(req.prompt);
      const imageUrls = referenceUrlList(Array.isArray(req.image_urls) ? req.image_urls : req.images !== undefined || req.image !== undefined ? [].concat(req.images || [], req.image || []) : req.image_urls, "image_urls");
      const metadata = req.metadata && typeof req.metadata === "object" && !Array.isArray(req.metadata) ? req.metadata : {};
      const referenceUrls = referenceUrlList(metadata.reference_urls !== undefined ? metadata.reference_urls : [].concat(req.reference_videos || [], req.videos || []), "metadata.reference_urls");
      if (imageUrls.length > 1) throw new Error("at most one reference image is supported per task");
      if (imageUrls.length && referenceUrls.length) throw new Error("image_urls and reference videos are mutually exclusive");

      const duration = normalizeDuration(req.seconds !== undefined ? req.seconds : req.duration);
      if (req.seconds !== undefined && duration === 0) throw new Error("seconds must be a number between 1 and " + MAX_VIDEO_DURATION);
      if (req.duration !== undefined && duration === 0) throw new Error("duration must be a number between 1 and " + MAX_VIDEO_DURATION);

      let resolution = normalizeVideoResolution(req.resolution !== undefined ? req.resolution : req.resolution_name);
      let ratio = trimmed(req.aspect_ratio !== undefined ? req.aspect_ratio : req.ratio);
      if (ratio === "auto" || ratio.toLowerCase() === "adaptive") ratio = "";
      const size = trimmed(req.size);
      if (size && size !== "auto") {
        const dimensions = /^(\d+)x(\d+)$/.exec(size);
        if (dimensions) {
          const width = Number(dimensions[1]);
          const height = Number(dimensions[2]);
          if (!ratio) ratio = nearestVideoRatio(width + ":" + height);
          if (!resolution) resolution = videoResolutionTier(width, height);
        } else if (!ratio) {
          ratio = nearestVideoRatio(size);
        }
        if (!ratio) throw new Error("size must be pixel dimensions such as 1280x720 or a ratio such as 16:9");
      }
      if (ratio && !VIDEO_RATIOS.includes(ratio)) throw new Error("aspect ratio must be one of " + VIDEO_RATIOS.join(", "));

      const audio = normalizeBool(req.audio !== undefined ? req.audio : req.generate_audio);
      const watermark = normalizeBool(req.watermark);

      const body = { model: model };
      if (prompt) body.prompt = prompt;
      if (imageUrls.length) body.image_urls = imageUrls;
      if (referenceUrls.length) body.metadata = { reference_urls: referenceUrls };
      if (ratio) body.aspect_ratio = ratio;
      if (resolution) body.resolution = resolution;
      if (duration > 0) body.duration = duration;
      if (audio !== undefined) body.audio = audio;
      if (watermark !== undefined) body.watermark = watermark;

      // wan2.6-flash routes to i2v/r2v upstreams only; a text-only prompt has
      // no upstream and ToAPIs rejects it, so fail fast with the actionable
      // alternative instead of a bare "invalid parameters".
      if (video && model.toLowerCase().includes("flash") && !imageUrls.length && !referenceUrls.length) {
        throw new Error(model + " does not support text-to-video; provide a public image URL (image_urls) or reference video, or use wan2.6 for text-to-video");
      }

      return {
        kind: "submit",
        model: model,
        action: imageUrls.length ? "image_to_video" : referenceUrls.length ? "reference_video" : "text_to_video",
        requestBody: body,
      };
    },
    render: function (ctx, task) {
      const data = task && task.data && typeof task.data === "object" && !Array.isArray(task.data) ? task.data : {};
      const statusMap = { queued: "queued", in_progress: "in_progress", completed: "completed", failed: "failed" };
      const status = statusMap[trimmed(data.status).toLowerCase()] || "unknown";
      const output = {
        id: trimmed(task && task.task_id) || trimmed(data.id),
        object: "video",
        model: trimmed(task && task.properties && task.properties.origin_model_name) || trimmed(data.model),
        status: status,
        progress: progressPercent(data.progress, task && task.progress),
        created_at: Number(task && task.created_at) || Number(data.created_at) || 0,
      };
      const completedAt = Number(task && (task.finished_at || task.updated_at)) || 0;
      if (completedAt > 0) output.completed_at = completedAt;
      if (status === "completed") {
        const url = firstResultUrl(data);
        if (url) output.video_url = url;
      }
      if (status === "failed") {
        const error = data.error && typeof data.error === "object" && !Array.isArray(data.error) ? data.error : {};
        output.error = { code: String(error.code || "video_generation_failed"), message: trimmed(error.message) || "video generation failed" };
      }
      return output;
    },
  },
};

// The host persists every poll body into task data. A completed ToAPIs task
// carries its media at result.data[].url for both image and video tasks;
// tolerate a terminal snapshot that stored the OpenAI-style data[].url
// spelling instead.
function resultImageUrls(task) {
  const data = task && task.data && typeof task.data === "object" && !Array.isArray(task.data) ? task.data : {};
  const result = data.result && typeof data.result === "object" && !Array.isArray(data.result) ? data.result : {};
  const urls = collectImageUrls(result.data);
  if (!urls.length) {
    for (const url of collectImageUrls(data.data)) urls.push(url);
  }
  return urls;
}

function collectImageUrls(entries) {
  const urls = [];
  if (!Array.isArray(entries)) return urls;
  for (const item of entries) {
    const url = item && typeof item === "object" ? trimmed(item.url) : "";
    if (url) urls.push(url);
  }
  return urls;
}

export function parseSubmitResponse(ctx, resp) {
  const body = resp && resp.body && typeof resp.body === "object" && !Array.isArray(resp.body) ? resp.body : {};
  const taskId = trimmed(body.id);
  if (!taskId) throw new Error("task_id is empty");
  return { taskId: taskId, taskData: body };
}

export function buildQueryRequest(ctx) {
  return {
    url: ctx.baseUrl + (isVideoModel(ctx) ? "/v1/videos/generations/" : "/v1/images/generations/") + encodeURIComponent(trimmed(ctx.taskId)),
    method: "GET",
    headers: { Accept: "application/json", Authorization: "Bearer " + ctx.apiKey },
  };
}

export function parseTaskResult(ctx, body) {
  const payload = body && typeof body === "object" && !Array.isArray(body) ? body : {};
  const status = trimmed(payload.status).toLowerCase();
  if (status === "queued" || status === "in_progress") {
    const result = { status: status === "queued" ? "QUEUED" : "IN_PROGRESS" };
    const progress = Number(payload.progress);
    if (Number.isFinite(progress) && progress >= 0 && progress < 100) result.progress = Math.floor(progress) + "%";
    return result;
  }
  if (status === "completed") {
    const result = { status: "SUCCESS", progress: "100%", url: firstResultUrl(payload) };
    const usage = payload.usage && typeof payload.usage === "object" && !Array.isArray(payload.usage) ? payload.usage : {};
    const completionTokens = Number(usage.output_tokens);
    if (Number.isFinite(completionTokens) && completionTokens > 0) result.completionTokens = Math.floor(completionTokens);
    const totalTokens = Number(usage.total_tokens);
    if (Number.isFinite(totalTokens) && totalTokens > 0) result.totalTokens = Math.floor(totalTokens);
    return result;
  }
  if (status === "failed") {
    const error = payload.error && typeof payload.error === "object" && !Array.isArray(payload.error) ? payload.error : {};
    return { status: "FAILURE", progress: "100%", reason: trimmed(error.message) || "generation failed" };
  }
  return { status: "UNKNOWN", reason: "unrecognized status: " + status };
}

function firstResultUrl(payload) {
  const result = payload.result && typeof payload.result === "object" && !Array.isArray(payload.result) ? payload.result : {};
  const urls = collectImageUrls(result.data);
  return urls.length ? urls[0] : "";
}

// The client-facing OpenAI video object carries progress as 0-100 integer.
function progressPercent(bodyProgress, taskProgress) {
  for (const value of [bodyProgress, taskProgress]) {
    const progress = Number(String(value === undefined || value === null ? "" : value).replace("%", ""));
    if (Number.isFinite(progress) && progress >= 0 && progress <= 100) return Math.floor(progress);
  }
  return 0;
}

export function buildSubmitRequest(ctx) {
  const video = isVideoModel(ctx);
  const body = Object.assign({}, ctx.requestBody || {});
  body.model = trimmed(ctx.upstreamModel) || trimmed(body.model);
  return {
    url: ctx.baseUrl + (video ? "/v1/videos/generations" : "/v1/images/generations"),
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json", Authorization: "Bearer " + ctx.apiKey },
    body: body,
    action: ctx.action,
  };
}

export function listArtifacts(task) {
  if (!task || task.status !== "SUCCESS") return [];
  return firstResultUrl(taskDataOf(task)) ? [{ key: "video", type: "video" }] : [];
}

// Serve the completed media through its ToAPIs result URL; the URL is the
// delivery surface, so the content proxy needs no extra upstream endpoint.
export function buildContentRequest(ctx) {
  if (ctx.artifactKey !== "video") throw new Error("artifact_not_found");
  const url = firstResultUrl(taskDataOf(ctx));
  if (!url) throw new Error("artifact_not_found");
  return { url: url, method: ctx.clientRequest.method, credentialless: true };
}

function taskDataOf(value) {
  return value && typeof value === "object" && !Array.isArray(value) ? value.data && typeof value.data === "object" && !Array.isArray(value.data) ? value.data : value : {};
}

export function extractUsage(ctx) {
  if (isVideoModel(ctx)) {
    const body = ctx.requestBody || {};
    const duration = normalizeDuration(body.duration);
    const facts = {
      seconds: duration > 0 ? duration : 5,
      resolution: normalizeVideoResolution(body.resolution) || "720p",
    };
    const audio = normalizeBool(body.audio);
    if (audio !== undefined) facts.audio = audio;
    return facts;
  }
  const n = Number((ctx.requestBody || {}).n);
  return { image_count: Number.isInteger(n) && n >= 1 ? n : 1 };
}
