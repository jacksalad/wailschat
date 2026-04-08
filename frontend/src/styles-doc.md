# WailsChat 自定义样式指南

> 本文档说明如何在 **Settings → Styles** 中编写自定义 CSS 样式，以覆盖系统默认样式。

---

## 核心机制

1. **最高优先级**：Custom Styles 中的 CSS 会直接注入到 `<head>` 底部，优先级高于所有默认样式和 Tailwind 类
2. **实时预览**：保存后立即生效，无需重启应用
3. **Reset to Default**：点击重置按钮可恢复内置默认样式

---

## CSS 变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `--chat-font-family` | 全局字体 | System Default |
| `--chat-font-size` | 全局字号 | 14px |
| `--chat-width` | 聊天消息区最大宽度 | 768px |

---

## 可定制 CSS 选择器参考

### 全局 & 深色模式

```css
html, body { }                  /* 根元素 - 控制整体背景和文字色 */
html.dark, html.dark body { }   /* 深色模式根元素 */
#app { }                        /* 主应用容器 */
```

### 侧边栏 (Sidebar)

```css
.sidebar { }                    /* 侧边栏整体 */
.bg-white\/50, .bg-slate-800\/50 { }  /* 侧边栏背景（半透明） */
.bg-blue-600 { }                /* 新建聊天按钮 */
.session-item { }               /* 会话列表项 */
.session-item.active { }        /* 当前选中会话项 */
```

### 聊天窗口 (ChatWindow)

```css
.messages-container { }         /* 消息区域容器 */
.chat-header { }                 /* Header 区域 */
.model-picker-btn { }           /* 模型选择器按钮 */
```

### 消息气泡 (MessageBubble)

```css
.bg-blue-600\/50 { }            /* 用户消息气泡 */
.bg-slate-100\/50, .bg-slate-700\/50 { }  /* AI 助手消息气泡 */
.typing-cursor { }               /* 流式输出打字光标 */
.message-actions { }             /* 操作按钮容器 */
.stats-popup { }                /* 性能统计弹出框 */
```

### 输入区域 (ChatInput)

```css
.chat-input-container { }       /* 输入框容器 */
.chat-input-textarea { }        /* 文本输入框 */
.send-btn { }                   /* 发送按钮 */
```

### Markdown & 代码

```css
.markdown-body { }               /* Markdown 整体 */
.markdown-body p, .markdown-body h1, .markdown-body h2 { }  /* 文本元素 */
.markdown-body blockquote { }   /* 引用块 */
.markdown-body :not(pre) > code { }  /* 行内代码 */
.code-block { }                 /* 代码块外层 */
.code-header { }                /* 代码块头部 */
.code-lang { }                  /* 代码语言标签 */
.copy-btn, .run-btn { }          /* 复制/运行按钮 */
.hljs { }                       /* highlight.js 语法高亮 */
```

### 滚动条

```css
::-webkit-scrollbar { }          /* 滚动条轨道 */
::-webkit-scrollbar-track { }
::-webkit-scrollbar-thumb { }   /* 滚动条滑块 */
::-webkit-scrollbar-thumb:hover { }
```

---

## 快速覆盖示例

```css
/* 更改用户消息颜色为紫色 */
.bg-blue-600\/50 { background-color: #8b5cf6 !important; }

/* 深色模式背景 */
html.dark, html.dark body { background-color: #0F172A !important; }

/* 自定义滚动条 */
::-webkit-scrollbar-thumb { background: #6366f1; border-radius: 4px; }

/* 代码块深色模式 */
html.dark .hljs { background: #1E293B !important; }
```

---

## 6 个内置主题 CSS 示例

> 复制以下任意代码块到 Settings → Styles，点击 Save 即可预览。

---

### 主题 1：奶油玻璃 Cream Glass

```css
/* === 奶油玻璃 Cream Glass === */
html, body {
  background: linear-gradient(135deg, #F8F6F2 0%, #EEF1F5 100%) !important;
  color: #2E2E38 !important;
}
html.dark, html.dark body {
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%) !important;
  color: #E8E8EC !important;
}
.flex.h-full.bg-white\/50, .bg-slate-800\/50 {
  background: rgba(255, 255, 255, 0.7) !important;
  backdrop-filter: blur(20px) !important;
}
html.dark .flex.h-full.bg-white\/50, html.dark .bg-slate-800\/50 {
  background: rgba(30, 30, 50, 0.8) !important;
  backdrop-filter: blur(20px) !important;
}
.bg-blue-600\/50 {
  background: linear-gradient(135deg, #7C9CF5 0%, #A5B4FC 100%) !important;
  border-radius: 20px !important;
}
.bg-slate-100\/50, html.dark .bg-slate-700\/50 {
  background: rgba(255, 255, 255, 0.6) !important;
  backdrop-filter: blur(10px) !important;
  border-radius: 20px !important;
}
html.dark .bg-slate-100\/50, html.dark .bg-slate-700\/50 {
  background: rgba(50, 50, 80, 0.6) !important;
}
.chat-input-textarea {
  background: rgba(255, 255, 255, 0.8) !important;
  border-color: #E5E7EB !important;
  border-radius: 16px !important;
}
html.dark .chat-input-textarea {
  background: rgba(30, 30, 50, 0.8) !important;
  border-color: #374151 !important;
}
button.bg-blue-600 {
  background: linear-gradient(135deg, #7C9CF5, #A5B4FC) !important;
  border-radius: 14px !important;
  box-shadow: 0 4px 12px rgba(124, 156, 245, 0.3) !important;
}
button.bg-blue-600:hover {
  background: linear-gradient(135deg, #6B8BF0, #95A4EE) !important;
}
::-webkit-scrollbar-thumb {
  background: rgba(124, 156, 245, 0.5) !important;
  border-radius: 4px !important;
}
::-webkit-scrollbar-thumb:hover {
  background: rgba(124, 156, 245, 0.8) !important;
}
.session-item:hover {
  background: rgba(124, 156, 245, 0.15) !important;
}
```

---

### 主题 2：多巴胺果冻 Dopamine Pop

```css
/* === 多巴胺果冻 Dopamine Pop === */
html, body { background: #FFF8F5 !important; color: #2D2D2D !important; }
html.dark, html.dark body { background: #1A1625 !important; color: #F0F0F0 !important; }
.bg-blue-600\/50 {
  background: linear-gradient(135deg, #FF6FAE 0%, #FF8A5B 100%) !important;
  border-radius: 22px 22px 4px 22px !important;
  box-shadow: 0 4px 16px rgba(255, 111, 174, 0.35) !important;
}
.bg-slate-100\/50, html.dark .bg-slate-700\/50 {
  background: linear-gradient(135deg, #E8F5F0 0%, #D4E9F0 100%) !important;
  border-radius: 22px 22px 22px 4px !important;
}
html.dark .bg-slate-100\/50, html.dark .bg-slate-700\/50 {
  background: linear-gradient(135deg, #2D3A4A 0%, #3D4A5A 100%) !important;
}
.flex.h-full.bg-white\/50 { background: #FFF8F5 !important; }
html.dark .flex.h-full.bg-white\/50, .bg-slate-800\/50 { background: #1A1625 !important; }
.chat-input-textarea {
  background: #FFFFFF !important;
  border: 2px solid #FFD93D !important;
  border-radius: 18px !important;
  box-shadow: 0 0 0 4px rgba(255, 217, 61, 0.15) !important;
}
html.dark .chat-input-textarea {
  background: #2D2D3A !important;
  border-color: #9B5CFF !important;
  box-shadow: 0 0 0 4px rgba(155, 92, 255, 0.15) !important;
}
button.bg-blue-600 {
  background: linear-gradient(135deg, #FFD93D 0%, #FF8A5B 100%) !important;
  border-radius: 14px !important;
  box-shadow: 0 4px 14px rgba(255, 217, 61, 0.4) !important;
  transition: all 0.2s ease !important;
}
button.bg-blue-600:hover {
  transform: translateY(-2px) scale(1.02) !important;
  box-shadow: 0 6px 20px rgba(255, 217, 61, 0.5) !important;
}
.session-item.active, .bg-blue-600\/50 {
  background: linear-gradient(135deg, #FF6FAE 0%, #FF8A5B 100%) !important;
}
.session-item:hover { background: rgba(255, 111, 174, 0.1) !important; }
::-webkit-scrollbar-thumb {
  background: linear-gradient(180deg, #FF6FAE, #4D96FF) !important;
  border-radius: 4px !important;
}
```

---

### 主题 3：赛博霓虹 Cyber Neon

```css
/* === 赛博霓虹 Cyber Neon === */
html, body { background: #0B0F1A !important; color: #E0E8F0 !important; }
#app > div { background: transparent !important; }
.flex.h-full.bg-white\/50, .bg-slate-800\/50 {
  background: rgba(11, 15, 26, 0.85) !important;
  backdrop-filter: blur(16px) !important;
  border-color: rgba(79, 209, 255, 0.15) !important;
}
.bg-blue-600\/50 {
  background: linear-gradient(135deg, rgba(79, 209, 255, 0.9), rgba(155, 92, 255, 0.9)) !important;
  border-radius: 4px 20px 20px 20px !important;
  box-shadow: 0 0 20px rgba(79, 209, 255, 0.4), 0 0 40px rgba(155, 92, 255, 0.2) !important;
  border: 1px solid rgba(79, 209, 255, 0.5) !important;
}
.bg-slate-100\/50, .bg-slate-700\/50 {
  background: rgba(18, 24, 42, 0.9) !important;
  border: 1px solid rgba(79, 209, 255, 0.2) !important;
  border-radius: 20px 4px 20px 20px !important;
  box-shadow: 0 0 15px rgba(79, 209, 255, 0.1) !important;
}
.chat-input-textarea {
  background: rgba(12, 18, 32, 0.95) !important;
  border: 1px solid rgba(79, 209, 255, 0.4) !important;
  border-radius: 16px !important;
  color: #E0E8F0 !important;
  box-shadow: 0 0 15px rgba(79, 209, 255, 0.15) !important;
}
.chat-input-textarea:focus {
  border-color: #4FD1FF !important;
  box-shadow: 0 0 25px rgba(79, 209, 255, 0.3), 0 0 50px rgba(155, 92, 255, 0.15) !important;
}
button.bg-blue-600 {
  background: linear-gradient(135deg, #4FD1FF, #9B5CFF) !important;
  border-radius: 12px !important;
  box-shadow: 0 0 20px rgba(79, 209, 255, 0.5), 0 0 40px rgba(155, 92, 255, 0.3) !important;
  transition: all 0.2s ease !important;
}
button.bg-blue-600:hover {
  box-shadow: 0 0 30px rgba(79, 209, 255, 0.7), 0 0 60px rgba(155, 92, 255, 0.5) !important;
  transform: scale(1.05) !important;
}
button.text-xs.px-2.py-1 {
  background: rgba(12, 18, 32, 0.9) !important;
  border: 1px solid rgba(79, 209, 255, 0.3) !important;
  color: #4FD1FF !important;
  text-shadow: 0 0 10px rgba(79, 209, 255, 0.5) !important;
}
.session-item:hover {
  background: rgba(79, 209, 255, 0.08) !important;
  border-color: rgba(79, 209, 255, 0.3) !important;
}
.session-item.active {
  background: linear-gradient(135deg, rgba(79, 209, 255, 0.2), rgba(155, 92, 255, 0.2)) !important;
  border-color: rgba(79, 209, 255, 0.5) !important;
  box-shadow: 0 0 15px rgba(79, 209, 255, 0.2) !important;
}
::-webkit-scrollbar-thumb {
  background: linear-gradient(180deg, #4FD1FF, #9B5CFF) !important;
  border-radius: 4px !important;
  box-shadow: 0 0 8px rgba(79, 209, 255, 0.5) !important;
}
.code-block {
  border: 1px solid rgba(79, 209, 255, 0.3) !important;
  box-shadow: 0 0 15px rgba(79, 209, 255, 0.1) !important;
}
.code-header { background: rgba(12, 18, 32, 0.95) !important; border-bottom: 1px solid rgba(79, 209, 255, 0.2) !important; }
.code-lang { color: #4FD1FF !important; text-shadow: 0 0 8px rgba(79, 209, 255, 0.5) !important; }
.markdown-body a { color: #4FD1FF !important; text-shadow: 0 0 8px rgba(79, 209, 255, 0.4) !important; }
.typing-cursor::after { color: #4FD1FF !important; text-shadow: 0 0 15px rgba(79, 209, 255, 0.8), 0 0 30px rgba(79, 209, 255, 0.4) !important; }
```

---

### 主题 4：深海极简 Deep Ocean

```css
/* === 深海极简 Deep Ocean === */
html, body { background: #0F172A !important; color: #CBD5E1 !important; }
html.dark, html.dark body { background: #0F172A !important; color: #CBD5E1 !important; }
.flex.h-full.bg-white\/50, .bg-slate-800\/50 { background: #111827 !important; border-color: #1E293B !important; }
.bg-blue-600\/50 {
  background: linear-gradient(135deg, #334155 0%, #475569 100%) !important;
  border-radius: 16px 16px 4px 16px !important;
}
.bg-slate-100\/50, .bg-slate-700\/50 {
  background: #1E293B !important;
  border-radius: 16px 16px 16px 4px !important;
  border: 1px solid #334155 !important;
}
.chat-input-textarea {
  background: #1E293B !important;
  border-color: #334155 !important;
  border-radius: 12px !important;
  color: #E2E8F0 !important;
}
.chat-input-textarea::placeholder { color: #64748B !important; }
.chat-input-textarea:focus {
  border-color: #38BDF8 !important;
  box-shadow: 0 0 0 3px rgba(56, 189, 248, 0.1) !important;
}
button.bg-blue-600 { background: #38BDF8 !important; border-radius: 10px !important; transition: all 0.15s ease !important; }
button.bg-blue-600:hover { background: #0EA5E9 !important; }
.session-item { border-radius: 10px !important; transition: all 0.15s ease !important; }
.session-item:hover { background: #1E293B !important; }
.session-item.active { background: #334155 !important; }
.session-item .text-sm { color: #E2E8F0 !important; }
.session-item .text-xs { color: #64748B !important; }
button.text-xs.px-2.py-1 { background: #1E293B !important; border: 1px solid #334155 !important; color: #38BDF8 !important; }
::-webkit-scrollbar-thumb { background: #334155 !important; border-radius: 4px !important; }
::-webkit-scrollbar-thumb:hover { background: #475569 !important; }
.code-block { background: #1E293B !important; border-color: #334155 !important; border-radius: 10px !important; }
.code-header { background: #111827 !important; border-bottom-color: #334155 !important; }
.code-lang { color: #38BDF8 !important; }
.hljs { background: #1E293B !important; color: #E2E8F0 !important; }
.markdown-body blockquote {
  border-left-color: #38BDF8 !important;
  color: #94A3B8 !important;
  background: rgba(56, 189, 248, 0.05) !important;
  padding: 8px 16px !important;
  border-radius: 0 8px 8px 0 !important;
}
.absolute.bottom-full.left-0.mb-2 { background: #1E293B !important; border: 1px solid #334155 !important; box-shadow: 0 10px 40px rgba(0, 0, 0, 0.4) !important; }
```

---

### 主题 5：日光纸感 Paper UI

```css
/* === 日光纸感 Paper UI === */
html, body { background: #F7F1E8 !important; color: #3A312B !important; }
html.dark, html.dark body { background: #2C2825 !important; color: #E8E4DC !important; }
.flex.h-full.bg-white\/50 { background: rgba(255, 253, 248, 0.9) !important; border-color: #E5DDD0 !important; }
.bg-slate-800\/50 { background: rgba(44, 40, 37, 0.9) !important; border-color: #3D3832 !important; }
.bg-blue-600\/50 {
  background: linear-gradient(135deg, #7A9E7E 0%, #8FB08F 100%) !important;
  border-radius: 20px 20px 4px 20px !important;
  box-shadow: 0 2px 8px rgba(122, 158, 126, 0.25) !important;
}
.bg-blue-600\/50.text-white { color: #FFFFFF !important; }
.bg-slate-100\/50, .bg-slate-700\/50 {
  background: #FFFDF8 !important;
  border: 1px solid #E8E0D4 !important;
  border-radius: 20px 20px 20px 4px !important;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05) !important;
}
html.dark .bg-slate-100\/50, html.dark .bg-slate-700\/50 { background: #3D3832 !important; border-color: #4D4842 !important; }
.chat-input-textarea {
  background: #FFFDF8 !important;
  border: 1px solid #D4C8B8 !important;
  border-radius: 16px !important;
  color: #3A312B !important;
  box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.06) !important;
}
html.dark .chat-input-textarea { background: #3D3832 !important; border-color: #4D4842 !important; color: #E8E4DC !important; }
.chat-input-textarea:focus {
  border-color: #7A9E7E !important;
  box-shadow: 0 0 0 3px rgba(122, 158, 126, 0.15), inset 0 1px 3px rgba(0, 0, 0, 0.06) !important;
}
button.bg-blue-600 { background: #7A9E7E !important; border-radius: 14px !important; box-shadow: 0 2px 8px rgba(122, 158, 126, 0.3) !important; }
button.bg-blue-600:hover { background: #6B8E6F !important; }
.session-item { border-radius: 12px !important; transition: all 0.15s ease !important; }
.session-item:hover { background: rgba(122, 158, 126, 0.1) !important; }
.session-item.active { background: rgba(122, 158, 126, 0.2) !important; }
.session-item .text-sm { color: #3A312B !important; }
html.dark .session-item .text-sm { color: #E8E4DC !important; }
::-webkit-scrollbar-thumb { background: #D4C8B8 !important; border-radius: 4px !important; }
::-webkit-scrollbar-thumb:hover { background: #C4B8A8 !important; }
html.dark ::-webkit-scrollbar-thumb { background: #4D4842 !important; }
.code-block { background: #FFFDF8 !important; border: 1px solid #E8E0D4 !important; border-radius: 12px !important; }
html.dark .code-block { background: #3D3832 !important; border-color: #4D4842 !important; }
.code-header { background: #F5EFE6 !important; border-bottom-color: #E8E0D4 !important; border-radius: 12px 12px 0 0 !important; }
html.dark .code-header { background: #35302A !important; border-bottom-color: #4D4842 !important; }
.code-lang { color: #7A9E7E !important; }
html.dark .code-lang { color: #8FB08F !important; }
.markdown-body blockquote { border-left-color: #C98B5F !important; color: #6B5D4D !important; background: rgba(201, 139, 95, 0.08) !important; border-radius: 0 8px 8px 0 !important; }
html.dark .markdown-body blockquote { color: #A89B8B !important; background: rgba(201, 139, 95, 0.1) !important; }
button.text-xs.px-2.py-1 { background: #FFFDF8 !important; border: 1px solid #D4C8B8 !important; color: #7A9E7E !important; font-weight: 500 !important; }
html.dark button.text-xs.px-2.py-1 { background: #3D3832 !important; border-color: #4D4842 !important; color: #8FB08F !important; }
```

---

### 主题 6：液态金属 Liquid Chrome

```css
/* === 液态金属 Liquid Chrome === */
html, body { background: #0E0E10 !important; color: #C9CED6 !important; }
html.dark, html.dark body { background: #0E0E10 !important; color: #C9CED6 !important; }
.flex.h-full.bg-white\/50, .bg-slate-800\/50 {
  background: linear-gradient(180deg, #1A1A1E 0%, #0E0E10 100%) !important;
  border-color: #2A2A30 !important;
}
.bg-blue-600\/50 {
  background: linear-gradient(135deg, #7CF7FF 0%, #A78BFA 50%, #F472B6 100%) !important;
  border-radius: 20px 20px 4px 20px !important;
  box-shadow: 0 4px 20px rgba(124, 247, 255, 0.25), 0 4px 40px rgba(167, 139, 250, 0.15) !important;
  color: #0E0E10 !important;
  font-weight: 500 !important;
}
.bg-slate-100\/50, .bg-slate-700\/50 {
  background: linear-gradient(135deg, #1E1E24 0%, #2A2A32 100%) !important;
  border: 1px solid #3A3A44 !important;
  border-radius: 20px 20px 20px 4px !important;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.05), 0 4px 16px rgba(0, 0, 0, 0.3) !important;
}
.chat-input-textarea {
  background: linear-gradient(135deg, #1A1A1E 0%, #0E0E10 100%) !important;
  border: 1px solid #3A3A44 !important;
  border-radius: 14px !important;
  color: #E8E8EC !important;
  box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.3), 0 0 20px rgba(124, 247, 255, 0.05) !important;
}
.chat-input-textarea:focus {
  border-color: #7CF7FF !important;
  box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.3), 0 0 25px rgba(124, 247, 255, 0.2) !important;
}
.chat-input-textarea::placeholder { color: #5A5A66 !important; }
button.bg-blue-600 {
  background: linear-gradient(135deg, #7CF7FF 0%, #A78BFA 100%) !important;
  border-radius: 12px !important;
  box-shadow: 0 4px 16px rgba(124, 247, 255, 0.3), 0 4px 32px rgba(167, 139, 250, 0.2) !important;
  transition: all 0.2s ease !important;
  color: #0E0E10 !important;
  font-weight: 600 !important;
}
button.bg-blue-600:hover {
  box-shadow: 0 6px 24px rgba(124, 247, 255, 0.5), 0 6px 40px rgba(167, 139, 250, 0.3) !important;
  transform: translateY(-2px) scale(1.03) !important;
  background: linear-gradient(135deg, #9CF9FF 0%, #B89BFF 100%) !important;
}
.session-item {
  border-radius: 12px !important;
  border: 1px solid transparent !important;
  transition: all 0.2s ease !important;
}
.session-item:hover { background: rgba(124, 247, 255, 0.05) !important; border-color: rgba(124, 247, 255, 0.2) !important; }
.session-item.active {
  background: linear-gradient(135deg, rgba(124, 247, 255, 0.15) 0%, rgba(167, 139, 250, 0.15) 100%) !important;
  border-color: rgba(124, 247, 255, 0.4) !important;
  box-shadow: 0 0 20px rgba(124, 247, 255, 0.15) !important;
}
::-webkit-scrollbar-thumb {
  background: linear-gradient(180deg, #7CF7FF, #A78BFA, #F472B6) !important;
  border-radius: 4px !important;
}
::-webkit-scrollbar-thumb:hover { box-shadow: 0 0 10px rgba(124, 247, 255, 0.5) !important; }
.code-block { border: 1px solid #3A3A44 !important; box-shadow: 0 4px 20px rgba(124, 247, 255, 0.08) !important; }
.code-header { background: linear-gradient(180deg, #1E1E24, #1A1A1E) !important; border-bottom: 1px solid #3A3A44 !important; }
.code-lang { background: linear-gradient(135deg, #7CF7FF, #A78BFA) !important; -webkit-background-clip: text !important; -webkit-text-fill-color: transparent !important; font-weight: 700 !important; }
```

---

## 注意事项

1. **使用 `!important`**：确保自定义样式能够覆盖 Tailwind 的内联样式
2. **深色模式**：同时设置 `html.dark` 内的样式以确保两种模式都生效
3. **透明背景**：部分组件使用 `bg-white/50` 或 `bg-slate-700/50` 格式的半透明背景，覆盖时需要考虑这一点
4. **保留圆角和过渡**：建议保留原有的 `border-radius` 和 `transition` 属性以维持视觉一致性
