# 智能插话（打断+补充）功能实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现 Agent 处理过程中的智能插话功能，支持打断重来和上下文补充两种模式，并根据用户输入智能判断意图。

**Architecture:** 通过引入可中断的上下文管理和双通道消息处理，使 Agent 能够在生成过程中接收新消息。使用语义分析自动区分打断意图（否定、纠正）和补充意图（附加信息），并相应地调整处理流程。

**Tech Stack:** Go 1.21+, React/TypeScript, WebSocket, 上下文语义分析

---

## 任务清单

### Task 1: 创建核心中断机制 - InterruptibleContext

**文件:**
- 创建: `internal/agent/interrupt.go`
- 测试: `internal/agent/interrupt_test.go`

**Step 1: 定义中断类型和接口**

```go
package agent

import (
	"context"
	"sync"
	"time"
)

// InterruptMode 插话模式
type InterruptMode string

const (
	InterruptNone   InterruptMode = ""           // 无中断
	InterruptCancel InterruptMode = "cancel"     // 打断重来
	InterruptAppend InterruptMode = "append"     // 补充上下文
)

// InterruptRequest 中断请求
type InterruptRequest struct {
	Message   *bus.InboundMessage
	Mode      InterruptMode
	Timestamp time.Time
}

// InterruptibleContext 可中断的上下文
type InterruptibleContext struct {
	ctx           context.Context
	cancel        context.CancelFunc
	mu            sync.RWMutex
	interrupts    []InterruptRequest
	onInterrupt   func(InterruptRequest)
	appendQueue   []*bus.InboundMessage
	parentBus     *bus.MessageBus
}

// NewInterruptibleContext 创建可中断上下文
func NewInterruptibleContext(parent context.Context, bus *bus.MessageBus) *InterruptibleContext {
	ctx, cancel := context.WithCancel(parent)
	return &InterruptibleContext{
		ctx:         ctx,
		cancel:      cancel,
		interrupts:  make([]InterruptRequest, 0),
		appendQueue: make([]*bus.InboundMessage, 0),
		parentBus:   bus,
	}
}

// RequestInterrupt 请求中断
func (ic *InterruptibleContext) RequestInterrupt(req InterruptRequest) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	ic.interrupts = append(ic.interrupts, req)

	if req.Mode == InterruptCancel {
		ic.cancel() // 取消当前操作
	}

	if ic.onInterrupt != nil {
		go ic.onInterrupt(req)
	}
}

// GetPendingAppends 获取待处理的补充消息
func (ic *InterruptibleContext) GetPendingAppends() []*bus.InboundMessage {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	result := ic.appendQueue
	ic.appendQueue = make([]*bus.InboundMessage, 0)
	return result
}

// Done 返回完成通道
func (ic *InterruptibleContext) Done() <-chan struct{} {
	return ic.ctx.Done()
}

// Err 返回错误
func (ic *InterruptibleContext) Err() error {
	return ic.ctx.Err()
}
```

**Step 2: 编写测试**

```go
// interrupt_test.go
func TestInterruptibleContext_Cancel(t *testing.T) {
	ctx := context.Background()
	bus := bus.NewMessageBus(10)
	ic := NewInterruptibleContext(ctx, bus)

	done := make(chan bool)
	go func() {
		<-ic.Done()
		done <- true
	}()

	ic.RequestInterrupt(InterruptRequest{
		Mode: InterruptCancel,
	})

	select {
	case <-done:
		// 成功取消
	case <-time.After(time.Second):
		t.Fatal("cancel did not work")
	}
}

func TestInterruptibleContext_Append(t *testing.T) {
	ctx := context.Background()
	bus := bus.NewMessageBus(10)
	ic := NewInterruptibleContext(ctx, bus)

	msg := &bus.InboundMessage{Content: "补充信息"}
	ic.RequestInterrupt(InterruptRequest{
		Message: msg,
		Mode:    InterruptAppend,
	})

	appends := ic.GetPendingAppends()
	if len(appends) != 1 {
		t.Fatalf("expected 1 append, got %d", len(appends))
	}
}
```

**Step 3: 运行测试确保失败**

```bash
cd /Users/lua/git/nanobot-go
go test ./internal/agent -run TestInterrupt -v
```

**Step 4: 实现完整功能**

补全上面的代码，添加缺失的方法。

**Step 5: 提交**

```bash
git add internal/agent/interrupt.go internal/agent/interrupt_test.go
git commit -m "feat(agent): add interruptible context for message interruption"
```

---

### Task 2: 实现意图分析器 - IntentAnalyzer

**文件:**
- 创建: `internal/agent/intent.go`
- 测试: `internal/agent/intent_test.go`

**Step 1: 定义意图类型**

```go
package agent

import (
	"strings"
	"regexp"
)

// UserIntent 用户意图类型
type UserIntent string

const (
	IntentContinue    UserIntent = "continue"     // 继续当前话题
	IntentCorrection  UserIntent = "correction"   // 纠正/否定
	IntentAppend      UserIntent = "append"       // 补充信息
	IntentNewTopic    UserIntent = "new_topic"    // 新话题
	IntentStop        UserIntent = "stop"         // 明确要求停止
)

// IntentResult 意图分析结果
type IntentResult struct {
	Intent       UserIntent
	Confidence   float64
	IsInterrupt  bool
	Explanation  string
}

// IntentAnalyzer 意图分析器
type IntentAnalyzer struct {
	// 打断关键词（否定、纠正）
	correctionPatterns []string
	// 补充关键词
	appendPatterns     []string
	// 停止关键词
	stopPatterns       []string
}

// NewIntentAnalyzer 创建分析器
func NewIntentAnalyzer() *IntentAnalyzer {
	return &IntentAnalyzer{
		correctionPatterns: []string{
			"不对", "错了", "不是这样", "改一下", "换成", "修改为",
			"no", "wrong", "incorrect", "change to", "use",
			"不是", "要的是", "应该是", "更正", "纠正",
			"等一下", "stop", "停止", "别", "不要",
		},
		appendPatterns: []string{
			"对了", "还有", "另外", "补充", "记得", "别忘了",
			"also", "plus", "add", "remember", "by the way",
			"顺便", "以及", "并且", "而且",
		},
		stopPatterns: []string{
			"停止生成", "不要生成了", "stop", "够了", "可以了",
			"cancel", "abort", "停",
		},
	}
}

// Analyze 分析用户输入意图
func (a *IntentAnalyzer) Analyze(input string, currentContext string) IntentResult {
	inputLower := strings.ToLower(strings.TrimSpace(input))

	// 1. 检查停止意图（最高优先级）
	for _, pattern := range a.stopPatterns {
		if strings.Contains(inputLower, strings.ToLower(pattern)) {
			return IntentResult{
				Intent:      IntentStop,
				Confidence:  0.95,
				IsInterrupt: true,
				Explanation: "检测到明确的停止请求",
			}
		}
	}

	// 2. 检查纠正意图
	for _, pattern := range a.correctionPatterns {
		if strings.Contains(inputLower, strings.ToLower(pattern)) {
			return IntentResult{
				Intent:      IntentCorrection,
				Confidence:  0.85,
				IsInterrupt: true,
				Explanation: "检测到纠正/否定意图",
			}
		}
	}

	// 3. 检查补充意图
	for _, pattern := range a.appendPatterns {
		if strings.Contains(inputLower, strings.ToLower(pattern)) {
			return IntentResult{
				Intent:      IntentAppend,
				Confidence:  0.80,
				IsInterrupt: false,
				Explanation: "检测到补充信息意图",
			}
		}
	}

	// 4. 长度启发式：短消息更可能是打断
	if len(input) < 20 {
		return IntentResult{
			Intent:      IntentCorrection,
			Confidence:  0.60,
			IsInterrupt: true,
			Explanation: "短消息倾向于打断",
		}
	}

	// 默认继续
	return IntentResult{
		Intent:      IntentContinue,
		Confidence:  0.70,
		IsInterrupt: false,
		Explanation: "未检测到特殊意图，继续当前话题",
	}
}
```

**Step 2: 编写测试**

```go
func TestIntentAnalyzer(t *testing.T) {
	analyzer := NewIntentAnalyzer()

	tests := []struct {
		name     string
		input    string
		expected UserIntent
		isInterrupt bool
	}{
		{"纠正", "不对，应该用 Go", IntentCorrection, true},
		{"补充", "对了，记得加上错误处理", IntentAppend, false},
		{"停止", "停止生成", IntentStop, true},
		{"继续", "详细的解释一下", IntentContinue, false},
		{"短消息打断", "用 Python", IntentCorrection, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.Analyze(tt.input, "")
			if result.Intent != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result.Intent)
			}
			if result.IsInterrupt != tt.isInterrupt {
				t.Errorf("isInterrupt expected %v, got %v", tt.isInterrupt, result.IsInterrupt)
			}
		})
	}
}
```

**Step 3: 运行测试**

```bash
go test ./internal/agent -run TestIntent -v
```

**Step 4: 提交**

```bash
git add internal/agent/intent.go internal/agent/intent_test.go
git commit -m "feat(agent): add intent analyzer for interruption detection"
```

---

### Task 3: 增强 AgentLoop 支持中断处理

**文件:**
- 修改: `internal/agent/loop.go`

**Step 1: 添加中断相关字段**

```go
// 在 AgentLoop struct 中添加
type AgentLoop struct {
	// ... 现有字段

	intentAnalyzer    *IntentAnalyzer
	currentIC         *InterruptibleContext
	icMu              sync.RWMutex
}
```

**Step 2: 修改 NewAgentLoop 初始化**

```go
func NewAgentLoop(...) *AgentLoop {
	// ... 现有代码
	loop := &AgentLoop{
		// ... 现有字段
		intentAnalyzer: NewIntentAnalyzer(),
	}
	// ... 后续代码
}
```

**Step 3: 添加中断处理方法**

```go
// HandleInterruption 处理插话请求
func (a *AgentLoop) HandleInterruption(msg *bus.InboundMessage) InterruptMode {
	a.icMu.RLock()
	ic := a.currentIC
	a.icMu.RUnlock()

	if ic == nil {
		// 没有在处理中的任务，作为普通消息处理
		return InterruptNone
	}

	// 分析意图
	intent := a.intentAnalyzer.Analyze(msg.Content, "")

	switch intent.Intent {
	case IntentStop, IntentCorrection:
		ic.RequestInterrupt(InterruptRequest{
			Message: msg,
			Mode:    InterruptCancel,
		})
		return InterruptCancel

	case IntentAppend:
		ic.RequestInterrupt(InterruptRequest{
			Message: msg,
			Mode:    InterruptAppend,
		})
		return InterruptAppend

	default:
		// 默认作为打断处理（保守策略）
		ic.RequestInterrupt(InterruptRequest{
			Message: msg,
			Mode:    InterruptCancel,
		})
		return InterruptCancel
	}
}
```

**Step 4: 修改 ProcessMessage 使用可中断上下文**

```go
func (a *AgentLoop) ProcessMessage(ctx context.Context, msg *bus.InboundMessage) (*bus.OutboundMessage, error) {
	// 创建可中断上下文
	ic := NewInterruptibleContext(ctx, a.Bus)

	a.icMu.Lock()
	a.currentIC = ic
	a.icMu.Unlock()

	defer func() {
		a.icMu.Lock()
		a.currentIC = nil
		a.icMu.Unlock()
	}()

	return a.processMessageWithIC(ic, msg)
}

func (a *AgentLoop) processMessageWithIC(ic *InterruptibleContext, msg *bus.InboundMessage) (*bus.OutboundMessage, error) {
	// 类似 processMessageWithCallbacks，但使用 ic 替代 ctx
	// ... 实现细节
}
```

**Step 5: 提交**

```bash
git add internal/agent/loop.go
git commit -m "feat(agent): enhance AgentLoop with interruption handling"
```

---

### Task 4: WebSocket 支持实时中断

**文件:**
- 修改: `internal/webui/websocket.go` (如果不存在则创建)
- 修改: `internal/webui/server.go`

**Step 1: 添加 WebSocket 消息类型**

```go
// WebSocketMessageType 消息类型
type WebSocketMessageType string

const (
	WSMessageTypeChat       WebSocketMessageType = "chat"
	WSMessageTypeInterrupt  WebSocketMessageType = "interrupt"
	WSMessageTypeStream     WebSocketMessageType = "stream"
	WSMessageTypeStatus     WebSocketMessageType = "status"
)

// WebSocketMessage WebSocket 消息结构
type WebSocketMessage struct {
	Type      WebSocketMessageType `json:"type"`
	Session   string               `json:"session,omitempty"`
	Content   string               `json:"content,omitempty"`
	Mode      string               `json:"mode,omitempty"` // "cancel" | "append"
	Timestamp int64                `json:"timestamp,omitempty"`
}
```

**Step 2: 修改 WebSocket 处理器支持中断**

```go
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// ... 现有升级逻辑

	for {
		var msg WebSocketMessage
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		switch msg.Type {
		case WSMessageTypeChat:
			// 普通消息，通过 bus 发送
			inbound := bus.NewInboundMessage("desktop", msg.Session, "user", msg.Content)
			s.messageBus.PublishInbound(inbound)

		case WSMessageTypeInterrupt:
			// 中断请求
			mode := InterruptCancel
			if msg.Mode == "append" {
				mode = InterruptAppend
			}

			inbound := bus.NewInboundMessage("desktop", msg.Session, "user", msg.Content)
			s.agentLoop.HandleInterruption(inbound)
		}
	}
}
```

**Step 3: 提交**

```bash
git add internal/webui/websocket.go internal/webui/server.go
git commit -m "feat(webui): add WebSocket support for real-time interruption"
```

---

### Task 5: 前端实现双模式发送

**文件:**
- 修改: `electron/src/renderer/views/ChatView.tsx`
- 修改/创建: `electron/src/renderer/hooks/useWebSocket.ts`

**Step 1: 扩展 WebSocket hook 支持中断**

```typescript
// useWebSocket.ts
interface WebSocketMessage {
  type: 'chat' | 'interrupt' | 'stream' | 'status';
  session?: string;
  content?: string;
  mode?: 'cancel' | 'append';
  timestamp?: number;
}

interface UseWebSocketReturn {
  sendMessage: (content: string) => void;
  sendInterrupt: (content: string, mode: 'cancel' | 'append') => void;
  isGenerating: boolean;
  // ... 其他
}

export function useWebSocket(sessionKey: string): UseWebSocketReturn {
  const [isGenerating, setIsGenerating] = useState(false);
  const ws = useRef<WebSocket | null>(null);

  const sendMessage = (content: string) => {
    ws.current?.send(JSON.stringify({
      type: 'chat',
      session: sessionKey,
      content,
      timestamp: Date.now()
    }));
  };

  const sendInterrupt = (content: string, mode: 'cancel' | 'append') => {
    ws.current?.send(JSON.stringify({
      type: 'interrupt',
      session: sessionKey,
      content,
      mode,
      timestamp: Date.now()
    }));

    if (mode === 'cancel') {
      setIsGenerating(false);
    }
  };

  // ... 连接管理和事件监听

  return { sendMessage, sendInterrupt, isGenerating };
}
```

**Step 2: 修改 ChatView 输入区域**

```tsx
// ChatView.tsx 输入区域
function Composer({ onSend, isGenerating }) {
  const [input, setInput] = useState('');
  const [mode, setMode] = useState<'normal' | 'append'>('normal');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim()) return;

    if (isGenerating) {
      // 正在生成时，根据模式发送中断或补充
      const interruptMode = mode === 'normal' ? 'cancel' : 'append';
      onSend(input, interruptMode);
    } else {
      onSend(input, 'normal');
    }

    setInput('');
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      if (e.shiftKey) {
        // Shift+Enter = 补充模式
        setMode('append');
      } else {
        setMode('normal');
      }
      handleSubmit(e);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="composer">
      {isGenerating && (
        <div className="interrupt-hint">
          <span className={mode === 'append' ? 'active' : ''}>
            Shift+Enter 补充上下文
          </span>
          <span className={mode === 'normal' ? 'active' : ''}>
            Enter 打断并重试
          </span>
        </div>
      )}

      <div className="input-row">
        <textarea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={isGenerating ? "可以补充信息或打断..." : "输入消息..."}
        />

        {isGenerating ? (
          <div className="dual-actions">
            <button
              type="submit"
              className="btn-append"
              onClick={() => setMode('append')}
            >
              补充
            </button>
            <button
              type="submit"
              className="btn-interrupt"
              onClick={() => setMode('normal')}
            >
              打断
            </button>
          </div>
        ) : (
          <button type="submit">发送</button>
        )}
      </div>
    </form>
  );
}
```

**Step 3: 样式更新**

```css
/* ChatView.css */
.interrupt-hint {
  display: flex;
  gap: 16px;
  padding: 8px 12px;
  background: var(--secondary);
  border-radius: 8px 8px 0 0;
  font-size: 12px;
  color: var(--foreground-muted);
}

.interrupt-hint .active {
  color: var(--primary);
  font-weight: 500;
}

.dual-actions {
  display: flex;
  gap: 8px;
}

.btn-append {
  background: var(--info);
  color: white;
}

.btn-interrupt {
  background: var(--warning);
  color: white;
}
```

**Step 4: 提交**

```bash
git add electron/src/renderer/hooks/useWebSocket.ts
git add electron/src/renderer/views/ChatView.tsx
git add electron/src/renderer/views/ChatView.css
git commit -m "feat(electron): add dual-mode send for interruption and append"
```

---

### Task 6: 流式响应中断支持

**文件:**
- 修改: `internal/providers/openai.go` (或其他 provider)

**Step 1: 检查并支持上下文取消**

```go
func (p *OpenAIProvider) StreamComplete(ctx context.Context, messages []Message, callback StreamCallback) error {
	// ... 现有代码

	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return err
	}
	defer stream.Close()

	for {
		select {
		case <-ctx.Done():
			// 上下文被取消，优雅退出
			callback.OnError(ctx.Err())
			return ctx.Err()

		default:
			response, err := stream.Recv()
			if err != nil {
				// ... 错误处理
				return err
			}

			// 处理响应 ...
			delta := response.Choices[0].Delta.Content
			callback.OnContent(delta)
		}
	}
}
```

**Step 2: 提交**

```bash
git add internal/providers/openai.go
git commit -m "feat(provider): support context cancellation in streaming"
```

---

### Task 7: 集成测试和端到端验证

**文件:**
- 创建: `e2e_test/interruption_test.go`

**Step 1: 编写 E2E 测试**

```go
package e2e_test

import (
	"testing"
	"time"
)

func TestInterruption_Cancel(t *testing.T) {
	// 1. 发送长消息
	// 2. 在响应完成前发送打断消息
	// 3. 验证原响应被中断
	// 4. 验证新响应开始
}

func TestInterruption_Append(t *testing.T) {
	// 1. 发送消息
	// 2. 在响应中发送补充消息
	// 3. 验证原响应继续，但包含补充上下文
}
```

**Step 2: 提交**

```bash
git add e2e_test/interruption_test.go
git commit -m "test(e2e): add interruption integration tests"
```

---

### Task 8: 文档更新

**文件:**
- 修改: `docs/ARCHITECTURE.md`
- 修改: `CHANGELOG.md`

**内容要点:**
- 中断机制架构说明
- WebSocket 协议扩展
- 意图分析算法说明
- 前端交互设计

**提交:**

```bash
git add docs/ARCHITECTURE.md CHANGELOG.md
git commit -m "docs: add interruption feature architecture documentation"
```

---

## 测试策略

### 单元测试
- `interrupt_test.go` - 中断上下文管理
- `intent_test.go` - 意图分析器

### 集成测试
- WebSocket 连接和消息类型
- AgentLoop 中断处理流程

### E2E 测试
- 前端到后端的完整中断流程
- 多种意图的识别准确率

### 手动测试清单
- [ ] 正常对话不受影响
- [ ] 打断后立即重新生成
- [ ] 补充后原生成继续但考虑新信息
- [ ] 快速连续发送多条消息
- [ ] 网络延迟情况下的表现

---

## 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 中断导致数据不一致 | 高 | 使用事务性上下文，确保状态回滚 |
| 意图误判 | 中 | 保守策略，不确定时默认打断 |
| 性能下降 | 中 | 异步处理中断，不阻塞主流程 |
| WebSocket 连接断开 | 中 | 实现重连机制和消息队列 |

---

## 发布计划

1. **阶段 1** (Task 1-3): 后端核心功能
2. **阶段 2** (Task 4-5): WebSocket 和前端
3. **阶段 3** (Task 6): Provider 支持
4. **阶段 4** (Task 7-8): 测试和文档
5. **Beta 发布**: 内部测试一周
6. **正式发布**: 合并到 main
