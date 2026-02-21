# 浏览器工具

基于 CDP 的浏览器自动化。

---

## 概述

浏览器工具使用 Chrome DevTools Protocol (CDP) 进行浏览器自动化。

### 要求

- 安装 Chrome/Chromium/Brave/Edge
- 启用 CDP 端口 (默认: 18800)

---

## 操作

### 状态

```bash
browser(action="status")
```

返回浏览器连接状态。

### 启动

```bash
browser(action="start")
```

以无头模式启动浏览器。

### 停止

```bash
browser(action="stop")
```

停止浏览器实例。

### 导航

```bash
browser(action="navigate", targetUrl="https://example.com")
```

### 快照

```bash
browser(action="snapshot")
```

返回 AI 可读格式的页面结构。

```bash
# 带限制
browser(action="snapshot", limit=50)

# Aria 格式
browser(action="snapshot", snapshotFormat="aria")
```

### 截图

```bash
browser(action="screenshot")
```

返回 PNG 截图。

```bash
# 完整页面
browser(action="screenshot", fullPage=true)
```

### 点击

```bash
browser(action="act", request={"kind": "click", "targetId": "element-id"})
```

### 输入

```bash
browser(action="act", request={"kind": "type", "targetId": "input-id", "text": "hello"})
```

### 等待

```bash
browser(action="act", request={"kind": "wait", "selector": "#element", "timeMs": 5000})
```

---

## 配置

### 默认端口

```bash
export CDP_PORT=18800
```

### 配置

```json
{
  "browser": {
    "enabled": true,
    "headless": false,
    "profiles": {
      "openclaw": {
        "cdpPort": 18800,
        "driver": "cdp"
      },
      "chrome": {
        "cdpPort": 18792,
        "cdpUrl": "http://127.0.0.1:18792"
      }
    }
  }
}
```

---

## 限制

- 需要 Chrome/Chromium (不支持 Firefox/Safari)
- JavaScript 密集型站点可能有 issues
- 限于 CDP 支持的功能
- 文件上传/下载需要额外设置

---

## 相关文档

- [工具概览](overview-zh.md)
- [Web 工具](web-zh.md)
