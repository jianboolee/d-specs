# 界面体验优化方案（对话框 + 三栏布局）

参考主流 AI 产品（ChatGPT / Claude / Notion AI 中间栏）打磨「AI 对话栏 + 参数画布」的体验。remixicon 已在 `main.ts` 引入，直接复用图标，不新增依赖。全部改动集中在 `web/src/` 前端，不改后端。

## 一、左右布局宽度 + 可拖动分隔条（`views/WorkspaceView.vue`）

现状：AI 栏 `width:42%`（`min 340 / max 620`），右栏画布 `flex:1`，分隔线只是 `.chat-head` 上的 `border-right`。

改动：
1. AI 栏改为**固定像素宽度**，默认 `440px`，范围 `min 360px / max 720px`。
2. 用 `ref` 记录宽度，并**持久化到 localStorage**（key 如 `dspecs.chatWidth`），刷新后保持用户拖动的结果。
3. 在 `.chat-col` 与 `SpecCanvas` 之间插入一个**分隔条**（约 6px 宽的 `div`），用原生 `pointerdown / pointermove / pointerup` 实现拖动：按下时记录起始位置，移动时更新宽度并用 `clamp` 限幅，松手结束；拖动中给分隔条加高亮色 + `user-select:none` + 全局 `cursor: col-resize`。
4. 移除 `.chat-head` 上原来的 `border-right`，视觉分隔交给分隔条。

## 二、输入框打磨（`components/chat/ChatPanel.vue`）

1. **自动增高**：textarea 监听 `input`，用 `scrollHeight` 动态设置高度，`max-height:148px` 后出现内部滚动（现有 `rows=1` 保留最小高度）。
2. **焦点光晕**：`.composer-box:focus-within` 在现有边框变主色基础上，加 `box-shadow` 光晕（浅色主色），提升焦点反馈。
3. **发送按钮**：圆形主色按钮，hover 上浮 + 轻微放大；`disabled` 时弱化；发送中切换为停止按钮（已有逻辑保留），图标用 `ri-arrow-up-line` / `ri-stop-fill`。
4. **空态快捷建议 chips**：在空态引导文字下方加 3~4 个可点击的建议指令（如「帮我建一张蓝牙耳机的参数表」「把电池容量改成 5000mAh」「给续航分组加上单位」），点击即填入输入框（`draft` 赋值并聚焦）。这是 ChatGPT 式空态的常见做法，能明显提升新用户上手体验。

## 三、消息气泡细节（`components/chat/ChatPanel.vue`）

1. **气泡圆角差异化**：assistant 气泡左上角用更小圆角、user 气泡右下角更小圆角（经典的「聊天气泡」不对称圆角），视觉更接近主流对话产品。
2. **间距与行高微调**：消息间距、气泡 `line-height`、AI 头像与首行对齐（`align-items: flex-start`）。
3. **打字动画保留并微调**：现有三点闪烁动画保留，颜色/节奏略微收敛。
4. 不强制加时间戳（主流产品默认也不逐条显示，避免过度改动）。

## 四、AI 顶栏新增「新对话」入口（`views/WorkspaceView.vue`）

1. 在 `.chat-head` 右侧加一个图标按钮「新对话」（`ri-chat-new-line`），点击调用 `chat.newSession()` 清空当前对话（保留表格数据不变），并给出轻提示。
2. 顶栏标题区保持「AI 助手 / 直接说你要怎么改」，按钮仅在有对话时高亮可用；空对话时按钮置灰。

## 五、全局设计令牌微调（`assets/main.css`）

补充少量令牌，供上述组件复用，并同步暗色主题：
- `--ds-shadow-sm` / `--ds-focus-ring`（焦点光晕颜色）
- 顺带把当前未使用的 `--ds-canvas-width` 注释说明清理，避免误导。

## 验证方式

改完后用浏览器在 `localhost:5173` 上验证：登录 → 打开「新星北斗 Y600C」表 → 检查（1）分隔条可拖动且刷新后宽度保持；（2）输入框自动增高、焦点光晕、发送按钮态；（3）空态建议 chips 点击可填入；（4）「新对话」按钮清空对话；（5）深浅色主题下视觉正常。由于改动集中在样式与交互、不含业务逻辑，`npm run type-check` 通过即可，不新增测试。

## 不改动的范围

- 侧边栏（`AppSidebar.vue`）已较完整，暂不动。
- 参数画布表格（`SpecCanvas.vue`）结构不动，仅因布局宽度变化而自然受益。
- 后端、数据协议、分享页均不改。