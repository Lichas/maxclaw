# Providers

当前仅支持 **OpenAI 兼容 API**。如需使用 Anthropic / Gemini / Groq 等非兼容模型，请通过 OpenAI 兼容网关（如 OpenRouter、vLLM、LiteLLM、自建兼容代理）转发。
已验证可直接使用 `minimax` 配置（MiniMax 官方 OpenAI 兼容接口）。

配置示例（OpenRouter）：

```json
{
  "providers": {
    "openrouter": {
      "apiKey": "YOUR_KEY",
      "apiBase": "https://openrouter.ai/api/v1"
    }
  },
  "agents": {
    "defaults": {
      "model": "openrouter/your-model"
    }
  }
}
```

MiniMax 配置示例：

```json
{
  "providers": {
    "minimax": {
      "apiKey": "YOUR_MINIMAX_KEY",
      "apiBase": "https://api.minimax.io/v1"
    }
  },
  "agents": {
    "defaults": {
      "model": "minimax/MiniMax-M2"
    }
  }
}
```

DashScope / Qwen 配置示例：

```json
{
  "providers": {
    "dashscope": {
      "apiKey": "YOUR_DASHSCOPE_KEY",
      "apiBase": "https://dashscope.aliyuncs.com/compatible-mode/v1"
    }
  },
  "agents": {
    "defaults": {
      "model": "qwen-max"
    }
  }
}
```

扩展新 provider（两步）：
1. 在 `internal/config/schema.go` 的 `ProvidersConfig` 增加配置字段。
2. 在 `internal/providers/registry.go` 追加 `ProviderSpec`（关键词与默认 API Base）。
