# Architecture

```text
Host websites / integ-life
        │ import @integ-life/feedback
        │ X-Project-Key + optional Bearer token
        ▼
discuss.integ.life ── userinfo ──► auth.integ.life
        │
        ▼
PostgreSQL (projects, comments, private guest identity, feedback)
```

一个 project 对应一个宿主站或一组共享内容命名空间。`resource` 对应具体页面/文章/实体，因此同一服务可安全承载主站及所有子站。

注册身份只接受 integ-auth 返回的 `sub`，不接受客户端自报 `user_id`。游客身份明确标记为 `registered: false`。公开模型没有邮箱字段，避免误泄漏。后续管理端应使用独立的管理凭据，而不是 publishable project key。

首版刻意不包含投票、通知、审核后台和实时 WebSocket；数据模型保留稳定 ID 与状态字段，可在不破坏 SDK 的前提下增量加入。
