# WailsChat 自定义样式指南

> 本文档说明如何在 **Settings → Styles** 中编写自定义 CSS 样式，以覆盖系统默认样式，打造专属视觉体验。

---

## 核心机制

1. **最高优先级**：Custom Styles 中的 CSS 会直接注入到 `<head>` 底部，优先级高于所有默认样式和 Tailwind 类
2. **实时预览**：保存后立即生效，无需重启应用
3. **Reset to Default**：点击重置按钮可恢复内置默认样式
4. **CSS 变量支持**：使用 CSS 变量可以更方便地进行主题定制

---

## CSS 变量系统

### 全局与颜色变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `--chat-font-family` | 全局字体 | System Default |
| `--chat-font-size` | 全局字号 | 14px |
| `--chat-width` | 聊天消息区最大宽度 | 768px |
| `--user-bubble-bg` | 用户消息气泡背景 | rgba(37, 99, 235, 0.5) |
| `--user-bubble-text` | 用户消息文字颜色 | #ffffff |
| `--ai-bubble-bg-light` | AI 消息气泡背景（浅色） | rgba(241, 245, 249, 0.5) |
| `--ai-bubble-bg-dark` | AI 消息气泡背景（深色） | rgba(51, 65, 85, 0.5) |
| `--ai-bubble-text` | AI 消息文字颜色 | inherit |
| `--sidebar-bg-light` | 侧边栏背景（浅色） | rgba(255, 255, 255, 0.5) |
| `--sidebar-bg-dark` | 侧边栏背景（深色） | rgba(30, 41, 59, 0.5) |
| `--input-bg-light` | 输入框背景（浅色） | #ffffff |
| `--input-bg-dark` | 输入框背景（深色） | #334155 |
| `--input-border-light` | 输入框边框（浅色） | #cbd5e1 |
| `--input-border-dark` | 输入框边框（深色） | #475569 |
| `--input-focus-border` | 输入框聚焦边框 | #3b82f6 |
| `--send-btn-bg` | 发送按钮背景 | #2563eb |
| `--send-btn-hover-bg` | 发送按钮悬停背景 | #1d4ed8 |
| `--stop-btn-bg` | 停止按钮背景 | #dc2626 |
| `--header-bg-light` | 头部背景（浅色） | rgba(255, 255, 255, 0.5) |
| `--header-bg-dark` | 头部背景（深色） | rgba(30, 41, 59, 0.5) |
| `--text-primary-light` | 主要文字颜色（浅色） | #1e293b |
| `--text-primary-dark` | 主要文字颜色（深色） | #e2e8f0 |
| `--text-secondary-light` | 次要文字颜色（浅色） | #64748b |
| `--text-secondary-dark` | 次要文字颜色（深色） | #94a3b8 |

### 布局与效果变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `--radius-sm` | 小圆角 | 0.25rem |
| `--radius-md` | 中圆角 | 0.5rem |
| `--radius-lg` | 大圆角 | 0.75rem |
| `--transition-speed` | 过渡速度 | 150ms |
| `--shadow-sm` | 小阴影 | 0 1px 2px 0 rgb(0 0 0 / 0.05) |
| `--shadow-md` | 中阴影 | 0 4px 6px -1px rgb(0 0 0 / 0.1) |

### Markdown 渲染变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `--md-font-family` | Markdown 正文字体 | inherit |
| `--md-code-font-family` | 代码字体 | Consolas, Monaco, monospace |
| `--md-line-height` | 行高 | 1.7 |
| `--md-h1-font-size` | 一级标题字号 | 1.75rem |
| `--md-h2-font-size` | 二级标题字号 | 1.5rem |
| `--md-h3-font-size` | 三级标题字号 | 1.25rem |
| `--md-code-bg` | 行内代码背景 | #e2e8f0 |
| `--md-code-color` | 行内代码颜色 | inherit |
| `--md-pre-bg` | 代码块背景 | #f1f5f9 |
| `--md-pre-font-size` | 代码块字号 | 0.875rem |
| `--md-blockquote-border-color` | 引用块左边框颜色 | #cbd5e1 |
| `--md-blockquote-color` | 引用文字颜色 | #64748b |
| `--md-link-color` | 链接颜色 | #3b82f6 |
| `--md-link-hover-color` | 链接悬停颜色 | #2563eb |
| `--md-table-width` | 表格宽度 | 100% |
| `--md-th-bg` | 表头背景 | #f1f5f9 |

---

## CSS 选择器参考

### 受保护的选择器 (不可覆盖)

```css
.settings-modal-overlay { }   /* 设置弹窗遮罩层 */
.settings-modal-content { }    /* 设置弹窗内容层 */
```

### 全局与深色模式

```css
html, body { }                  /* 根元素 */
html.dark, html.dark body { }   /* 深色模式 */
#app { }                        /* 主应用容器 */
.app-container { }              /* 应用容器 */
```

### 侧边栏

```css
.sidebar { }                     /* 侧边栏容器 */
.sidebar-container { }            /* 侧边栏整体 */
.sidebar-header { }               /* 侧边栏头部 */
.new-chat-btn { }                /* 新建聊天按钮 */
.session-list { }                 /* 会话列表 */
.session-item { }                /* 会话项 */
.session-item:hover { }          /* 会话项悬停 */
.session-item.active { }         /* 当前选中 */
```

### 聊天窗口

```css
.chat-container { }              /* 聊天容器 */
.chat-header { }                 /* Header 区域 */
.chat-title { }                  /* 聊天标题 */
.model-picker-btn { }            /* 模型选择器按钮 */
.empty-state { }                 /* 空状态 */
.chat-messages { }               /* 消息列表容器 */
.loading-indicator { }           /* 加载指示器 */
.thinking-bubble { }             /* 思考中气泡 */
```

### 消息气泡

```css
.message-wrapper { }             /* 消息包装器 */
.message-bubble { }              /* 消息气泡 */
.user-bubble { }                 /* 用户消息气泡 */
.ai-bubble { }                   /* AI 消息气泡 */
.message-content { }             /* 消息内容 */
.message-actions { }             /* 消息操作按钮 */
.copy-btn, .retry-btn, .stats-btn { }  /* 操作按钮 */
```

### MCP 工具调用

```css
.tool-calls-panel { }            /* 工具调用面板 */
.tool-call-item { }              /* 单个工具调用 */
.tool-call-name { }              /* 工具名称 */
.tool-call-status { }            /* 调用状态 */
```

### 输入区域

```css
.chat-input-area { }             /* 输入区域容器 */
.input-textarea { }              /* 文本输入框 */
.input-textarea:focus { }        /* 聚焦状态 */
.image-preview-container { }     /* 图片预览 */
.send-btn { }                    /* 发送按钮 */
.stop-btn { }                    /* 停止按钮 */
```

### Markdown 与代码

```css
.markdown-body { }                /* Markdown 整体 */
.markdown-body h1-h6 { }          /* 标题 */
.markdown-body p { }              /* 段落 */
.markdown-body a { }              /* 链接 */
.markdown-body ul, ol { }         /* 列表 */
.markdown-body input[type="checkbox"] { }  /* 复选框 */
.markdown-body blockquote { }     /* 引用块 */
.markdown-body table { }          /* 表格 */
.markdown-body :not(pre) > code { }  /* 行内代码 */
.markdown-body .code-block { }   /* 代码块 */
.code-header { }                  /* 代码块头部 */
.code-lang { }                    /* 语言标签 */
.hljs { }                         /* highlight.js */
```

### 滚动条

```css
::-webkit-scrollbar { }          /* 滚动条轨道 */
::-webkit-scrollbar-track { }    /* 滚动条滑道 */
::-webkit-scrollbar-thumb { }     /* 滚动条滑块 */
```

---

## 快速覆盖示例

```css
/* 使用 CSS 变量快速修改主题色 */
:root {
  --send-btn-bg: #8b5cf6;
  --send-btn-hover-bg: #7c3aed;
  --user-bubble-bg: rgba(139, 92, 246, 0.5);
}

/* 深色模式背景 */
html.dark, html.dark body {
  background-color: #0F172A !important;
}

/* 自定义滚动条 */
::-webkit-scrollbar-thumb {
  background: linear-gradient(180deg, #6366f1, #8b5cf6);
  border-radius: 4px;
}

/* 代码块深色模式 */
html.dark .hljs { background: #1E293B !important; }

/* 玻璃态效果 */
.sidebar-container, .chat-header, .chat-input-area {
  backdrop-filter: blur(20px) !important;
}

/* 圆形消息气泡 */
.user-bubble, .ai-bubble {
  border-radius: 24px !important;
}
```

---

## 引用块样式参考

```css
/* 基础引用块变量 */
:root {
  --md-blockquote-border-width: 3px;
  --md-blockquote-border-color: #cbd5e1;
  --md-blockquote-bg: transparent;
  --md-blockquote-color: #64748b;
  --md-blockquote-padding: 0 0 0 1rem;
}

/* 自定义引用块样式 */
.markdown-body blockquote {
  border-left: 4px solid;
  border-image: linear-gradient(180deg, #667eea, #764ba2) 1;
  background: linear-gradient(90deg, rgba(102, 126, 234, 0.08) 0%, transparent 100%);
  padding: 1rem 1.25rem;
  border-radius: 0 8px 8px 0;
}
```

---

## 主题示例文件

本应用提供了 5 个精选主题，放在 `frontend/theme-css/` 目录下：

| 主题 | 文件 | 风格 |
|------|------|------|
| 赛博霓虹 | `cyber-neon.css` | 深色 + 霓虹发光 |
| 极光幻彩 | `aurora-gradient.css` | 渐变动画 |
| 紫罗兰之夜 | `violet-night.css` | 紫色优雅 |
| 极光绿光 | `aurora-green.css` | 绿色科技 |
| 液态金属 | `liquid-chrome.css` | 金属质感 |

将主题文件内容复制到 **Settings → Styles** 即可使用。

---

## 注意事项

1. **使用 `!important`**：确保自定义样式能够覆盖 Tailwind 的内联样式
2. **深色模式**：同时设置 `html.dark` 内的样式以确保两种模式都生效
3. **透明背景**：部分组件使用半透明背景 (`bg-white/50`)，覆盖时需要考虑这一点
4. **保留圆角和过渡**：建议保留原有的 `border-radius` 和 `transition` 属性
5. **CSS 变量**：使用 CSS 变量可以更方便地进行主题定制
6. **避免修改全局布局**：不要覆盖 `#app > div`、`.flex.h-screen` 等全局布局选择器
