# 健康检查和故障转移

监控 LLM 健康状况并自动故障转移。

---

## 概述

健康监控通过以下方式确保可靠的 LLM 服务：
- 定期健康检查
- 故障时自动故障转移
- 手动恢复选项

---

## 配置

```bash
export LLM_HEALTH_CHECK=1
export LLM_HEALTH_INTERVAL=1h
export LLM_HEALTH_FAILURE_THRESHOLD=3
```

```json
{
  "health": {
    "enabled": true,
    "interval": "1h",
    "failure_threshold": 3,
    "auto_failover": true
  }
}
```

---

## 工作原理

### 健康检查

1. 向 LLM 发送测试提示
2. 测量响应时间
3. 检查错误
4. 记录结果

### 故障检测

- 响应超时
- API 错误
- 连续失败 > 阈值

### 故障转移流程

1. 检测故障
2. 切换到备用提供商
3. 通知用户 (可选)
4. 继续操作

---

## 命令

```bash
# 检查健康状态
ocg llmhealth --action status

# 手动故障转移
ocg llmhealth --action failover --provider anthropic

# 重置为主提供商
ocg llmhealth --action reset

# 查看事件
ocg llmhealth --action events

# 测试特定提供商
ocg llmhealth --action test --provider openai
```

---

## 带回退的提供商

| 主提供商 | 回退 |
|----------|------|
| OpenAI | Anthropic |
| Anthropic | Google Gemini |
| Google | OpenAI |
| MiniMax | Ollama (如可用) |

---

## 相关文档

- [高级功能概览](overview-zh.md)
- [LLM Providers 概览](../../06-llm-providers/overview-zh.md)
