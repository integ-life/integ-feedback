# Integ Feedback

可独立部署、供 Integ Life 各站点共享的评论与用户反馈服务。它提供 HTTP API、无框架依赖的 TypeScript client，以及一个可选的轻量嵌入组件。

生产 API 的固定地址为 `https://discuss.integ.life`。

## 能力

- 按 `project + resource` 隔离评论线程，支持回复和游标分页。
- 已登录用户：前端传 integ-auth access token；服务端调用 OIDC `userinfo` 验证并展示身份。
- 游客：名字、邮箱均可选；默认显示 `Guest`，邮箱仅用于后台联系且不出现在响应里。
- 结构化反馈：`idea`、`issue`、`question`、`other`。
- 每个站点使用 publishable project key；服务端用 allowlist CORS 限制来源。
- 评论正文按纯文本处理；内置组件使用 `textContent`，不渲染用户 HTML。

## 本地启动

```bash
createdb feedback
psql "$DATABASE_URL" -f migrations/001_init.sql
go run ./cmd/server
```

先创建 project 与 key（数据库只保存 key 的 SHA-256）：

```sql
INSERT INTO projects(slug,name) VALUES ('integ-life','Integ Life');
INSERT INTO project_keys(project_id,key_hash)
SELECT id,encode(digest('pk_dev_replace_me','sha256'),'hex') FROM projects WHERE slug='integ-life';
```

## Web library

```ts
import { FeedbackClient, mountFeedback } from "@integ-life/feedback";

const config = {
  apiUrl: "https://discuss.integ.life",
  projectKey: "pk_live_...",
  // 返回 integ-auth access token；游客时返回 undefined
  getAccessToken: () => auth.accessToken,
};

const client = new FeedbackClient(config);
await client.createComment({
  resource: `${location.pathname}${location.search}`,
  body: "很有帮助！",
  guest: { name: "游客昵称", email: "optional@example.com" },
});

mountFeedback({ ...config, element: document.querySelector("#comments")!, resource: location.pathname });
```

`projectKey` 用于识别站点，不是秘密；安全边界是 CORS、OIDC token 校验、速率限制和部署层防滥用。生产环境应在网关增加按 IP/project 的 rate limit，并配置数据库备份。

完整接口见 [docs/api.md](docs/api.md)，架构决策见 [docs/architecture.md](docs/architecture.md)。

## 生产部署

服务默认只监听本机 `127.0.0.1:8385`，由 Caddy 在 `discuss.integ.life` 终止 TLS 并反向代理。参考配置位于 `deploy/production/`。
