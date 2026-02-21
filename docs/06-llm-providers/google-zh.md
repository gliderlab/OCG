# Google Gemini

配置 Google Gemini 作为 LLM 提供商，包括原生音频 realtime。

---

## 配置

### 环境变量

```bash
export GOOGLE_API_KEY="..."
export GOOGLE_MODEL="gemini-2.5-flash"
export GOOGLE_BASE_URL="https://generativelanguage.googleapis.com/v1"
```

### 配置文件

```json
{
  "llm": {
    "provider": "google",
    "model": "gemini-2.5-flash",
    "temperature": 0.7
  }
}
```

---

## Realtime（原生音频）

OCG Google realtime 适配器现已支持：

- 文本 + 音频流式传输
- PCM 音频上行（`SendAudio`）
- 音频下行 WAV 输出回调
- 函数调用 + 工具响应回合
- 输入/输出转录回调
- VAD/会话/使用量回调

推荐原生音频模型示例：

- `models/gemini-2.5-flash-native-audio-preview-12-2025`

---

## 函数调用（Realtime）

工具 schema 转换为 Gemini 函数声明（含参数）。

流程：

1. 模型发出工具调用
2. 回调处理业务逻辑
3. 通过 `SendToolResponse` 发送结果
4. 模型继续生成（包括音频响应）

---

## 可用模型（常用）

| 模型 | 上下文 | 描述 |
|------|--------|------|
| gemini-2.5-flash | 1M | 快速、大上下文 |
| gemini-2.5-pro | 2M | 高能力 |
| gemini-1.5-flash | 1M | 稳定选项 |
| gemini-1.5-pro | 2M | 高质量 |

Realtime 原生模型可能使用 `models/...` 下不同的命名。

---

## 注意

- Gemini 原生音频模型应在音频模式下运行。
- 非原生音频模型使用文本模式。
- 如遇 websocket 负载错误，请验证模型 + 模式是否匹配。

---

## Realtime 可调参数

以下参数可在 `RealtimeConfig` 中设置以微调 Live 行为：

### 生成参数
| 参数 | 类型 | 描述 |
|------|------|------|
| `temperature` | float | 随机性 (0-2) |
| `topP` | float | 核采样 |
| `topK` | float | Top-k 采样 |
| `maxTokens` | int32 | 最大输出 token |
| `seed` | int32 | 复现种子 |
| `includeThoughts` | bool | 在响应中包含思考 |
| `thinkingBudget` | int32 | 思考 token 预算 |

### 音频/语音
| 参数 | 类型 | 描述 |
|------|------|------|
| `voice` | string | 预建语音名称（如 "Kore"） |
| `speechLanguageCode` | string | 输出音频语言（如 "en-US"） |
| `inputAudioTranscription` | bool | 启用输入转录 |
| `outputAudioTranscription` | bool | 启用输出转录 |

### VAD（语音活动检测）
| 参数 | 类型 | 描述 |
|------|------|------|
| `explicitVAD` | bool | 使用显式 VAD 信号 |
| `autoVADDisabled` | *bool | 禁用自动 VAD |
| `vadStartSensitivity` | string | 起始灵敏度（"low"/"high"） |
| `vadEndSensitivity` | string | 结束灵敏度（"low"/"high"） |
| `vadPrefixPaddingMs` | int32 | 前缀填充（毫秒） |
| `vadSilenceDurationMs` | int32 | 静音阈值（毫秒） |
| `vadActivityHandling` | string | "start_of_activity_interrupts" / "no_interruption" |
| `vadTurnCoverage` | string | "turn_includes_only_activity" / "turn_includes_all_input" |

### 会话/上下文
| 参数 | 类型 | 描述 |
|------|------|------|
| `sessionResumption` | bool | 启用会话恢复 |
| `sessionResumptionHandle` | string | 恢复句柄 |
| `sessionResumptionTransparent` | bool | 透明恢复 |
| `contextWindowCompression` | bool | 启用压缩 |
| `contextCompressionTriggerTokens` | int64 | 触发阈值 |
| `contextCompressionTargetTokens` | int64 | 压缩后目标 |
| `mediaResolution` | string | "low"/"medium"/"high" |

### 主动性
| 参数 | 类型 | 描述 |
|------|------|------|
| `proactiveAudio` | *bool | 启用主动音频 |

---

## 使用示例

### 代码调用
```go
affective := true
proactive := false
cfg := llm.RealtimeConfig{
    Model:                      "models/gemini-2.5-flash-native-audio-preview-12-2025",
    Voice:                      "Kore",
    EnableAffectiveDialog:       &affective,
    ProactiveAudio:             &proactive,
    InputAudioTranscription:    true,
    OutputAudioTranscription:   true,
    VADStartSensitivity:        "low",
    VADEndSensitivity:         "low",
    VADSilenceDurationMs:        120,
    MediaResolution:             "medium",
}
```

### 命令切换（模态）
| 命令 | 行为 |
|------|------|
| `/live <text>` | 强制 Google Live（文本输入） |
| `/voice <text>` | 强制 Google Live（文本输入） |
| `/audio <text>` | 强制 Google Live（文本输入） |
| `/text <text>` | 强制 HTTP LLM（文本输入） |
| `/http <text>` | 强制 HTTP LLM（文本输入） |
| `/live-audio-file <path>` | 发送音频文件到 Live |

### 调试命令
- `/debug live` - 显示活动的 Live 会话和连接状态
- `/debug live <sessionKey>` - 显示指定会话详情
