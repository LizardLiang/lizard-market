---
name: agent-browser-advanced
description: >-
  Advanced agent-browser CLI patterns for AI-driven browser automation.
  Activate when the user needs headless browser automation, web scraping,
  E2E testing, network mocking, multi-tab workflows, or any task involving
  programmatic Chrome control via the agent-browser CLI. Covers batch
  execution, semantic selectors, HAR recording, device emulation,
  cookie/auth flows, and production-grade automation strategies.
---

# Agent-Browser: Advanced Usage Guide

You are an expert in **agent-browser**, a fast native Rust CLI for headless browser automation designed for AI agents. You use this knowledge to help users build robust, production-grade browser automation workflows.

**Repository:** https://github.com/vercel-labs/agent-browser

## Quick Reference

```bash
# Install
npm install -g agent-browser && agent-browser install

# Linux system deps
agent-browser install --with-deps

# Update
agent-browser upgrade
```

---

## 1. Semantic Selection Strategy

Always prefer semantic selectors over CSS/XPath. They are resilient to UI refactors.

### The Ref-Based Flow (Recommended for AI Agents)

```bash
# Step 1: Get accessibility tree with element references
agent-browser snapshot

# Output includes refs like @e1, @e2, @e3...
# Step 2: Act on refs directly
agent-browser click @e3
agent-browser fill @e7 "user@example.com"
```

**Why refs?** They map to the accessibility tree, meaning you interact with what users see — not implementation details.

### The Find-Based Flow (Stable Across Sessions)

```bash
# By ARIA role (most resilient)
agent-browser find role button click --name "Submit"
agent-browser find role textbox fill "search term" --name "Search"
agent-browser find role link click --name "Dashboard"

# By label (great for forms)
agent-browser find label "Email" fill "test@example.com"
agent-browser find label "Password" fill "s3cret"

# By test ID (when available)
agent-browser find testid "checkout-btn" click

# By text content
agent-browser find text "Sign In" click
agent-browser find text "Accept Cookies" --exact click

# By position (last resort)
agent-browser find first ".card" click
agent-browser find nth 2 "button" click
agent-browser find last "a" text
```

### Selector Priority (Best → Worst)

| Priority | Selector Type | Resilience | Example |
|----------|--------------|------------|---------|
| 1 | `@ref` from snapshot | Session-bound | `@e5` |
| 2 | `find role` with `--name` | Very high | `find role button click --name "Save"` |
| 3 | `find testid` | High | `find testid "submit-btn" click` |
| 4 | `find label` | High | `find label "Email" fill "..."` |
| 5 | `find text` with `--exact` | Medium | `find text "Login" --exact click` |
| 6 | CSS selector | Low | `#login-btn` |
| 7 | XPath | Lowest | `xpath://div[@class='form']` |

---

## 2. Batch Execution for Multi-Step Workflows

Batch mode executes multiple commands in a single process, eliminating per-command startup overhead. This is **critical** for performance in automation pipelines.

### Basic Batch

```bash
echo '[
  ["open", "https://app.example.com/login"],
  ["find", "label", "Email", "fill", "admin@example.com"],
  ["find", "label", "Password", "fill", "password123"],
  ["find", "role", "button", "click", "--name", "Sign In"],
  ["wait", "--url", "**/dashboard"],
  ["screenshot", "logged-in.png"]
]' | agent-browser batch --json
```

### Batch with Bail (Stop on First Error)

```bash
echo '[
  ["open", "https://checkout.example.com"],
  ["find", "testid", "add-to-cart", "click"],
  ["wait", "--text", "Added to cart"],
  ["find", "role", "link", "click", "--name", "Checkout"],
  ["find", "label", "Card Number", "fill", "4242424242424242"],
  ["find", "role", "button", "click", "--name", "Pay Now"],
  ["wait", "--text", "Payment confirmed"]
]' | agent-browser batch --json --bail
```

**Use `--bail`** when each step depends on the previous one. Without it, all commands run regardless of failures.

### Batch from File

```bash
# Save workflow to file
cat > login-flow.json << 'EOF'
[
  ["open", "https://app.example.com"],
  ["wait", "--load", "networkidle"],
  ["snapshot", "-i"],
  ["find", "role", "button", "click", "--name", "Get Started"]
]
EOF

agent-browser batch --json < login-flow.json
```

---

## 3. Network Mocking & API Testing

### Mock API Responses

```bash
# Mock a REST endpoint
agent-browser network route "https://api.example.com/users" \
  --body '{"users": [{"id": 1, "name": "Mock User"}]}'

# Block analytics/tracking
agent-browser network route "https://analytics.example.com/**" --abort
agent-browser network route "https://ads.example.com/**" --abort

# Then navigate — mocked responses are served automatically
agent-browser open "https://app.example.com"
```

### Inspect Network Traffic

```bash
# View all tracked requests
agent-browser network requests

# Filter by pattern
agent-browser network requests --filter "/api/"
agent-browser network requests --filter "graphql"
```

### HAR Recording for Performance Analysis

```bash
# Start recording
agent-browser network har start

# Perform the workflow
agent-browser open "https://app.example.com"
agent-browser find role button click --name "Load Data"
agent-browser wait --load networkidle

# Save recording
agent-browser network har stop recording.har
```

Use HAR files to:
- Analyze page load waterfall
- Identify slow API calls
- Debug request/response payloads
- Share reproducible network traces

### Remove Routes

```bash
# Remove specific route
agent-browser network unroute "https://api.example.com/users"

# Remove all routes
agent-browser network unroute
```

---

## 4. Authentication & Session Management

### Cookie-Based Auth

```bash
# Set auth cookie directly (skip login flow)
agent-browser cookies set "session" "abc123def456"
agent-browser cookies set "auth_token" "eyJhbGciOiJIUzI1NiJ9..."

# Verify cookies
agent-browser cookies

# Navigate to authenticated page
agent-browser open "https://app.example.com/dashboard"
```

### Basic Auth

```bash
agent-browser set credentials "admin" "password123"
agent-browser open "https://protected.example.com"
```

### localStorage Token Flow

```bash
# Set JWT in localStorage (common SPA pattern)
agent-browser open "https://app.example.com"
agent-browser eval "localStorage.setItem('token', 'eyJhbG...')"
agent-browser eval "localStorage.setItem('user', JSON.stringify({id: 1, role: 'admin'}))"

# Reload to pick up token
agent-browser open "https://app.example.com/dashboard"
```

### Full Login Flow with Session Persistence

```bash
echo '[
  ["open", "https://app.example.com/login"],
  ["find", "label", "Email", "fill", "user@example.com"],
  ["find", "label", "Password", "fill", "password"],
  ["find", "role", "button", "click", "--name", "Log In"],
  ["wait", "--url", "**/dashboard"],
  ["cookies"]
]' | agent-browser batch --json --bail
```

---

## 5. Multi-Tab Orchestration

### Parallel Page Comparison

```bash
# Open multiple tabs
agent-browser tab new "https://staging.example.com"
agent-browser tab new "https://production.example.com"

# Switch between tabs and capture state
agent-browser tab 1
agent-browser screenshot "staging.png"

agent-browser tab 2
agent-browser screenshot "production.png"
```

### Cross-Tab Data Transfer

```bash
# Tab 1: Extract data
agent-browser tab 1
agent-browser get text "#order-id"    # Returns: "ORD-12345"

# Tab 2: Use extracted data
agent-browser tab new "https://admin.example.com/orders"
agent-browser find label "Order ID" fill "ORD-12345"
agent-browser find role button click --name "Search"
```

### Tab Lifecycle

```bash
agent-browser tab              # List all open tabs
agent-browser tab new           # New blank tab
agent-browser tab new "url"     # New tab with URL
agent-browser tab 3             # Switch to tab 3
agent-browser tab close         # Close current tab
agent-browser tab close 2       # Close specific tab
agent-browser window new        # New browser window
```

---

## 6. Wait Strategies for Dynamic Content

### Progressive Wait Strategy

```bash
# 1. Wait for navigation
agent-browser wait --url "**/results"

# 2. Wait for network to settle
agent-browser wait --load networkidle

# 3. Wait for specific content
agent-browser wait --text "Results loaded"

# 4. Wait for element visibility
agent-browser wait "#data-table"

# 5. Wait for element to disappear (loading spinner)
agent-browser wait "#spinner" --state hidden

# 6. Wait for JS condition (complex state)
agent-browser wait --fn "document.querySelectorAll('.row').length > 10"
agent-browser wait --fn "window.__APP_STATE__.loaded === true"
```

### Wait with Timeout Patterns

```bash
# Wait for slow operations
agent-browser wait --fn "document.querySelector('#export-link')?.href"

# Polling pattern: wait for async process
agent-browser wait --fn "!document.body.innerText.includes('Processing...')"
```

### Load States Explained

| State | When Complete | Use Case |
|-------|--------------|----------|
| `load` | Window `load` event fires | Basic page loads |
| `domcontentloaded` | DOM fully parsed | SPAs with client rendering |
| `networkidle` | No network activity for 500ms | Pages with async data fetching |

---

## 7. Device Emulation & Responsive Testing

### Mobile Testing

```bash
# Emulate specific device
agent-browser set device "iPhone 14"
agent-browser open "https://app.example.com"
agent-browser screenshot "mobile-view.png"

# Custom viewport
agent-browser set viewport 375 812 3    # iPhone-like with 3x retina
agent-browser set viewport 768 1024     # iPad-like
agent-browser set viewport 1920 1080    # Desktop
```

### Geolocation Testing

```bash
# Set location (latitude, longitude)
agent-browser set geo 40.7128 -74.0060    # New York
agent-browser set geo 35.6762 139.6503    # Tokyo
agent-browser set geo 51.5074 -0.1278     # London

agent-browser open "https://maps.example.com"
agent-browser screenshot "location-test.png"
```

### Dark Mode Testing

```bash
agent-browser set media dark
agent-browser open "https://app.example.com"
agent-browser screenshot "dark-mode.png"

agent-browser set media light
agent-browser open "https://app.example.com"
agent-browser screenshot "light-mode.png"
```

### Offline Mode

```bash
# Simulate offline
agent-browser set offline on
agent-browser open "https://app.example.com"
agent-browser screenshot "offline-fallback.png"

# Back online
agent-browser set offline off
```

---

## 8. Screenshot & Visual Verification

### Screenshot Variants

```bash
# Basic viewport screenshot
agent-browser screenshot "page.png"

# Full-page (scrolled)
agent-browser screenshot --full "full-page.png"

# Annotated (with element labels for AI analysis)
agent-browser screenshot --annotate "annotated.png"

# Custom format and quality
agent-browser screenshot --screenshot-format jpeg --screenshot-quality 80 "compressed.jpg"

# Custom output directory
agent-browser screenshot --screenshot-dir ./screenshots "test-run.png"
```

### PDF Generation

```bash
agent-browser pdf "report.pdf"
```

### AI-Driven Visual Verification Loop

```bash
# 1. Take annotated screenshot
agent-browser screenshot --annotate "state.png"

# 2. AI analyzes the screenshot to understand current state
# 3. AI decides next action based on visual context
# 4. Repeat until goal is reached

# This pattern is the core loop for vision-based agents
```

---

## 9. Frame Navigation (iframes)

### Working with Embedded Content

```bash
# Enter an iframe
agent-browser frame "#payment-iframe"
agent-browser find label "Card Number" fill "4242424242424242"
agent-browser find label "Expiry" fill "12/28"
agent-browser find label "CVC" fill "123"

# Return to main page
agent-browser frame main
agent-browser find role button click --name "Complete Purchase"
```

### Nested Frames

```bash
# Enter outer frame
agent-browser frame "#outer-frame"
# Enter inner frame (relative to current)
agent-browser frame "#inner-frame"

# Back to main
agent-browser frame main
```

---

## 10. JavaScript Execution Patterns

### Extract Structured Data

```bash
# Get page data as JSON
agent-browser eval "JSON.stringify(window.__NEXT_DATA__)"

# Extract table data
agent-browser eval "JSON.stringify([...document.querySelectorAll('tr')].map(r => [...r.cells].map(c => c.textContent)))"

# Check application state
agent-browser eval "window.__APP_STATE__?.user?.role"
```

### Modify Page State

```bash
# Trigger client-side navigation
agent-browser eval "window.history.pushState({}, '', '/new-route')"

# Dispatch custom events
agent-browser eval "document.dispatchEvent(new Event('custom-event'))"

# Modify DOM for testing
agent-browser eval "document.querySelector('#feature-flag').dataset.enabled = 'true'"
```

### Base64 Encoded Scripts (for Complex JS)

```bash
# Encode complex scripts to avoid shell escaping issues
agent-browser eval "Y29uc29sZS5sb2coJ2hlbGxvJyk=" -b
```

### Piped Input

```bash
echo "document.title" | agent-browser eval --stdin
```

---

## 11. Dialog Handling

```bash
# Pre-configure dialog handling before triggering
agent-browser dialog accept              # Accept next alert/confirm
agent-browser dialog accept "response"   # Accept prompt with text
agent-browser dialog dismiss             # Dismiss/cancel

# Example: Delete confirmation flow
agent-browser dialog accept              # Pre-accept the confirm()
agent-browser find role button click --name "Delete Account"
```

**Warning:** Always set up dialog handling BEFORE triggering the action that causes the dialog.

---

## 12. CDP Integration

```bash
# Get CDP WebSocket URL for external tools
agent-browser get cdp-url

# Connect to existing browser instance
agent-browser connect 9222
```

Use CDP integration to:
- Attach Chrome DevTools for debugging
- Connect Puppeteer/Playwright to the same session
- Use specialized CDP tools alongside agent-browser

---

## 13. Production Automation Patterns

### E2E Test Flow

```bash
#!/bin/bash
set -e

# Setup
agent-browser set viewport 1280 720

# Login
agent-browser open "https://staging.example.com/login"
agent-browser find label "Email" fill "$TEST_USER"
agent-browser find label "Password" fill "$TEST_PASS"
agent-browser find role button click --name "Sign In"
agent-browser wait --url "**/dashboard"

# Verify dashboard loaded
agent-browser wait --text "Welcome"
agent-browser screenshot "01-dashboard.png"

# Create resource
agent-browser find role button click --name "New Project"
agent-browser find label "Project Name" fill "Test Project $(date +%s)"
agent-browser find role button click --name "Create"
agent-browser wait --text "Project created"
agent-browser screenshot "02-project-created.png"

# Cleanup
agent-browser close
```

### Web Scraping with Pagination

```bash
#!/bin/bash
PAGE=1
while true; do
  agent-browser open "https://example.com/listings?page=$PAGE"
  agent-browser wait --load networkidle

  # Extract data
  DATA=$(agent-browser eval "JSON.stringify([...document.querySelectorAll('.listing')].map(e => ({title: e.querySelector('h2').textContent, price: e.querySelector('.price').textContent})))")
  echo "$DATA" >> results.jsonl

  # Check for next page
  HAS_NEXT=$(agent-browser eval "!!document.querySelector('.next-page:not([disabled])')")
  if [ "$HAS_NEXT" != "true" ]; then
    break
  fi

  agent-browser find role link click --name "Next"
  PAGE=$((PAGE + 1))
done
```

### Visual Regression Testing

```bash
#!/bin/bash
PAGES=("/" "/about" "/pricing" "/docs")

agent-browser set viewport 1280 720

for PAGE in "${PAGES[@]}"; do
  SAFE_NAME=$(echo "$PAGE" | tr '/' '_')
  agent-browser open "https://app.example.com$PAGE"
  agent-browser wait --load networkidle
  agent-browser screenshot --full "baseline${SAFE_NAME}.png"
done

agent-browser close
```

### Form Fuzzing / Input Testing

```bash
#!/bin/bash
INPUTS=("" " " "a]b" "<script>alert(1)</script>" "' OR 1=1 --" "$(python3 -c 'print("A"*10000)')")

for INPUT in "${INPUTS[@]}"; do
  agent-browser open "https://app.example.com/search"
  agent-browser find role textbox fill "$INPUT" --name "Search"
  agent-browser find role button click --name "Search"
  agent-browser wait --load networkidle
  agent-browser screenshot "fuzz-$(date +%s%N).png"
done
```

### Monitor / Health Check

```bash
#!/bin/bash
agent-browser open "https://app.example.com"
agent-browser wait --load networkidle

TITLE=$(agent-browser get title)
URL=$(agent-browser get url)

if echo "$TITLE" | grep -qi "error\|maintenance\|down"; then
  echo "ALERT: Page title indicates issue: $TITLE"
  agent-browser screenshot "alert-$(date +%s).png"
  exit 1
fi

echo "OK: $URL loaded with title: $TITLE"
agent-browser close
```

---

## 14. Keyboard & Mouse Precision

### Keyboard Shortcuts

```bash
# Select all + copy
agent-browser press "Control+a"
agent-browser press "Control+c"

# Undo/redo
agent-browser press "Control+z"
agent-browser press "Control+Shift+z"

# Tab navigation
agent-browser press "Tab"
agent-browser press "Shift+Tab"

# Type with real keystrokes (triggers key events)
agent-browser keyboard type "Hello World"

# Insert text without key events (faster, no input events)
agent-browser keyboard inserttext "Bulk text content"
```

### Mouse Precision

```bash
# Click at exact coordinates
agent-browser mouse move 500 300
agent-browser mouse down
agent-browser mouse up

# Right-click context menu
agent-browser mouse move 500 300
agent-browser mouse down right
agent-browser mouse up right

# Drag operation via mouse
agent-browser mouse move 100 200
agent-browser mouse down
agent-browser mouse move 400 200
agent-browser mouse up

# Scroll wheel
agent-browser mouse wheel 300      # Scroll down
agent-browser mouse wheel -300     # Scroll up
agent-browser mouse wheel 0 200   # Scroll right
```

---

## 15. Troubleshooting Patterns

### Page Not Loading

```bash
# Check current state
agent-browser get url
agent-browser get title
agent-browser screenshot "debug.png"
```

### Element Not Found

```bash
# Get accessibility tree to understand page structure
agent-browser snapshot

# Check if element is in an iframe
agent-browser eval "document.querySelectorAll('iframe').length"

# Check if element is hidden
agent-browser is visible "#target-element"
```

### Timing Issues

```bash
# Always wait for network to settle before interacting
agent-browser wait --load networkidle

# Wait for specific JS condition instead of arbitrary delays
agent-browser wait --fn "document.querySelector('#app').__vue__.$store.state.loaded"
```

### Network Debugging

```bash
# Record everything
agent-browser network har start
# ... do the thing ...
agent-browser network har stop debug.har

# Check what requests were made
agent-browser network requests --filter "api"
```

---

## Command Quick Reference

| Category | Command | Key Flags |
|----------|---------|-----------|
| Navigate | `open <url>` | |
| Snapshot | `snapshot` | `-i` (inline) |
| Click | `click <sel>` | `--new-tab` |
| Fill | `fill <sel> <text>` | |
| Find | `find <type> <val> <action>` | `--name`, `--exact` |
| Wait | `wait` | `--text`, `--url`, `--load`, `--fn`, `--state` |
| Screenshot | `screenshot [path]` | `--full`, `--annotate`, `--screenshot-format` |
| Batch | `batch` | `--json`, `--bail` |
| Network | `network route <url>` | `--abort`, `--body` |
| HAR | `network har start/stop` | |
| Cookies | `cookies set <n> <v>` | |
| Storage | `storage local set <k> <v>` | |
| Tabs | `tab new [url]` | |
| Eval | `eval <js>` | `-b`, `--stdin` |
| Viewport | `set viewport <w> <h>` | `[scale]` |
| Device | `set device <name>` | |
| Close | `close` | |
