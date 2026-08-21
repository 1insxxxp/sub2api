# 自定义菜单打开方式设计

## 背景

当前所有自定义菜单都会进入 `/custom/:id`，并由 `CustomPageView` 使用 iframe 嵌入外部页面。“充值卡网（无需登录）”因此也运行在 iframe 中。充值卡站的支付宝渠道返回跳转链接后，会尝试在当前浏览上下文打开支付宝收银台；当前上下文是 iframe，而支付宝页面不支持第三方嵌套，导致二维码页空白或打不开。

## 目标

管理员可以为每个自定义菜单选择：

- `embedded`：继续在站内 iframe 中展示；
- `new_tab`：直接在浏览器新标签页中打开原始 URL。

部署后把“充值卡网（无需登录）”配置为 `new_tab`，其他现有菜单保持原行为。

## 数据模型与兼容性

在 `CustomMenuItem` 中新增可选字段 `open_mode`，允许值为 `embedded` 和 `new_tab`。

- 旧数据缺少该字段时按 `embedded` 处理，不需要数据迁移；
- 新建菜单默认使用 `embedded`；
- 后端拒绝未知值；
- `md:<slug>` Markdown 页面固定使用 `embedded`，不允许配置为 `new_tab`。

## 管理端

“系统设置 → 自定义菜单页面”的每个菜单项新增“打开方式”选择框：

- 站内嵌入；
- 新标签页。

Markdown 页面禁用新标签页选项。保存时把 `open_mode` 随现有 `custom_menu_items` 一并提交。

## 用户端导航

侧边栏构建导航项时：

- `embedded` 继续生成 `/custom/:id` 的 Vue Router 链接；
- `new_tab` 渲染普通 `<a>`，使用原始 URL、`target="_blank"` 和 `rel="noopener noreferrer"`；
- 新标签页模式不调用 `buildEmbeddedUrl`，不会把用户 ID、登录 Token、主题或来源地址附加到外部 URL；
- 移动端点击后仍关闭侧边栏。

`CustomPageView` 保持原有嵌入和 Markdown 展示能力，旧书签 `/custom/:id` 仍可访问。

## 安全与错误处理

- 后端继续校验 URL 必须为绝对 HTTP(S) 地址；
- 后端校验 `open_mode` 枚举，非法值返回 400；
- 新标签页链接使用 `noopener noreferrer`，防止外部页面访问来源窗口；
- 仅新标签页模式不传递嵌入鉴权参数，避免向外部站点暴露站内 Token。

## 测试

采用测试驱动方式覆盖：

1. 后端接受 `embedded` / `new_tab`，兼容空值并拒绝未知值；
2. Markdown 菜单拒绝 `new_tab`；
3. 管理端新建菜单默认 `embedded`，选择结果会被保存；
4. 侧边栏对 `embedded` 渲染站内路由，对 `new_tab` 渲染安全的外链；
5. 用户端、管理员个人区和管理员菜单中的自定义项行为一致；
6. 现有自定义菜单和设置测试保持通过。

## 上线配置

代码部署完成后，将公开设置中的“充值卡网（无需登录）”菜单项更新为 `open_mode: "new_tab"`。该配置更新只影响菜单打开方式，不修改支付渠道和订单数据。
