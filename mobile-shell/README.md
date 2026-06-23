# Sub2API Mobile Shell

`mobile-shell` 是一个最小化 uni-app WebView 壳，用于在 HarmonyOS、Android 和 iOS App 中加载已部署的 Sub2API Web 前端。

当前阶段已落实：

1. 新增独立 uni-app 壳目录。
2. 首次启动时由用户输入 Sub2API 域名。
3. 保存域名到本地存储后，通过 `<web-view>` 加载线上 Sub2API 前端。
4. 提供基础加载态、失败重试和返回键处理。
5. 用户可通过原生导航栏右侧“域名”按钮重新输入 Sub2API 域名。

支付、OAuth、外部链接拦截和原生能力桥接属于下一阶段，需确认后再做。

## 目录

```text
mobile-shell/
  App.vue
  main.ts
  manifest.json
  pages.json
  package.json
  tsconfig.json
  env.d.ts
  src/config.ts
  pages/index/index.vue
  scripts/validate-json.mjs
```

## 用户如何配置 Sub2API 地址

首次打开 App 时，会显示“请输入 Sub2API 域名”的启动页。点击“保存并打开”后，手机上应看到“已保存域名”的 toast；如果没有看到 toast，说明按钮事件没有触发，需要重新打包安装最新版本。

用户可以输入：

```text
sub2api.example.com
```

或完整地址：

```text
https://sub2api.example.com/
```

未填写协议时，壳会默认补全为 `https://`。

保存后地址会写入本地存储：

```text
sub2api_mobile_app_url
```

后续启动 App 会直接读取该地址并打开 WebView。

## 用户如何重新输入域名

WebView 页面打开后，原生导航栏右侧有“域名”按钮。

点击后会回到域名输入页：

- 保存新地址：立即加载新 Sub2API 站点，并覆盖本地保存值。
- 取消：继续使用当前已保存地址。

如果页面加载失败，也会出现“更换域名”按钮。

## 建议的服务端部署方式

建议保持 Sub2API 前端和后端 API 同源部署，例如：

```text
https://sub2api.example.com/
https://sub2api.example.com/api/v1
```

这样现有前端默认的 `/api/v1` API base URL 可以继续工作，减少 CORS 和 Cookie/凭据问题。

## 当前行为

- App 首次启动显示域名输入页。
- 保存地址后加载 WebView。
- HBuilderX 内置浏览器/H5 预览不会直接嵌入目标站点，而是显示提示页；因为 H5 的 `web-view` 本质是 iframe，遇到 `X-Frame-Options: DENY` 或 `frame-ancestors 'none'` 会被浏览器拒绝。
- Android、iOS、HarmonyOS App 端使用原生 WebView，应以运行到设备/模拟器作为真实验证方式。
- 加载中显示简单 loading 覆盖层。
- 加载失败显示重试和更换域名按钮。
- 移动端返回键：
  - 在域名输入页且已有保存地址时，返回 WebView。
  - 在 WebView 页时，优先调用 WebView 后退。
- `@message` 仅记录日志，暂不处理支付/OAuth/外部链接。

## 本地校验

```bash
cd D:/sub2api/mobile-shell
node scripts/validate-json.mjs
```

如果安装了依赖，可再运行：

```bash
pnpm install
pnpm typecheck
```

实际三端打包建议使用 HBuilderX / uni-app 工具链完成，并在 HarmonyOS、Android、iOS 真机上分别验证。

## 后续步骤四待确认

下一阶段建议处理：

- 支付链接外部打开。
- OAuth 登录外部浏览器/回跳策略。
- 文档链接、第三方链接打开策略。
- Web 页和 App 壳之间的 `postMessage` 通信协议。
- 下载、复制、分享、扫码等原生能力桥接。
