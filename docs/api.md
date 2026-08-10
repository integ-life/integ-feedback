# HTTP API

生产 Base URL：`https://discuss.integ.life`。

所有 `/v1/*` 请求必须带 `X-Project-Key`。登录用户另带 `Authorization: Bearer <integ-auth access token>`。JSON 错误格式为 `{"error":{"code":"...","message":"..."}}`。

| Method | Path | 用途 |
|---|---|---|
| GET | `/v1/comments?resource=...&limit=30&after=...` | 获取线程 |
| POST | `/v1/comments` | 发表评论或回复 |
| DELETE | `/v1/comments/{id}` | 登录用户删除自己的评论 |
| POST | `/v1/feedback` | 提交反馈 |

发表评论：

```json
{"resource":"/blog/a","body":"正文","parent_id":"","guest_name":"可选","guest_email":"可选且私密"}
```

提交反馈：

```json
{"resource":"/settings","kind":"issue","body":"保存后没有提示","guest_name":"","guest_email":""}
```

`resource` 是宿主站自行定义的稳定内容标识（建议 pathname 或业务实体 URN），最大 500 字符。正文最大 10,000 字符。
