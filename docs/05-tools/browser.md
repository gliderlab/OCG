# Browser Tool

CDP-based browser automation.

---

## Overview

The browser tool uses Chrome DevTools Protocol (CDP) for browser automation.

### Requirements

- Chrome/Chromium/Brave/Edge installed
- CDP port enabled (default: 18800)

---

## Actions

### Status

```bash
browser(action="status")
```

Returns browser connection status.

### Start

```bash
browser(action="start")
```

Starts browser in headless mode.

### Stop

```bash
browser(action="stop")
```

Stops browser instance.

### Navigate

```bash
browser(action="navigate", targetUrl="https://example.com")
```

### Snapshot

```bash
browser(action="snapshot")
```

Returns page structure in AI-readable format.

```bash
# With limit
browser(action="snapshot", limit=50)

# Aria format
browser(action="snapshot", snapshotFormat="aria")
```

### Screenshot

```bash
browser(action="screenshot")
```

Returns PNG screenshot.

```bash
# Full page
browser(action="screenshot", fullPage=true)
```

### Click

```bash
browser(action="act", request={"kind": "click", "targetId": "element-id"})
```

### Type

```bash
browser(action="act", request={"kind": "type", "targetId": "input-id", "text": "hello"})
```

### Wait

```bash
browser(action="act", request={"kind": "wait", "selector": "#element", "timeMs": 5000})
```

---

## Configuration

### Default Port

```bash
export CDP_PORT=18800
```

### Profiles

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

## Limitations

- Requires Chrome/Chromium (not Firefox/Safari)
- JavaScript-heavy sites may have issues
- Limited to CDP-supported features
- File upload/download requires additional setup

---

## See Also

- [Tools Overview](../overview.md)
- [Web Tools](web.md)
