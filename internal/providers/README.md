# Providers

当前 Provider 运行时分为两类：

- **原生官方 SDK**：`openai/*` 走 `github.com/openai/openai-go`，`anthropic/*` 走 `github.com/anthropics/anthropic-sdk-go`
- **OpenAI 兼容接口**：OpenRouter、DeepSeek、DashScope、Groq、MiniMax、vLLM 等继续走现有兼容层

Anthropic 默认 API Base：`https://api.anthropic.com`
OpenAI 默认 API Base：`https://api.openai.com/v1`
MiniMax 已验证可直接使用官方 OpenAI 兼容接口。

Anthropic 配置示例：

```json
{
  "providers": {
    "anthropic": {
      "apiKey": "YOUR_ANTHROPIC_KEY"
    }
  },
  "agents": {
    "defaults": {
      "model": "anthropic/claude-sonnet-4-5"
    }
  }
}
```

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

智谱 GLM（编码套餐端点）配置示例：

```json
{
  "providers": {
    "zhipu": {
      "apiKey": "YOUR_ZHIPU_KEY",
      "apiBase": "https://open.bigmodel.cn/api/coding/paas/v4"
    }
  },
  "agents": {
    "defaults": {
      "model": "glm-4.5"
    }
  }
}
```

扩展新 provider（两步）：
1. 在 `internal/config/schema.go` 的 `ProvidersConfig` 增加配置字段。
2. 在 `internal/providers/registry.go` 追加 `ProviderSpec`（关键词与默认 API Base）。
