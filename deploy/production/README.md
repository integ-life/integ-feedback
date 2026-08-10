# discuss.integ.life deployment

1. 将 Linux binary 安装为 `/usr/local/bin/integ-feedback`。
2. 创建不可登录的 `integ-feedback` 系统用户。
3. 把环境变量写入 root-only 的 `/etc/integ-feedback.env`。
4. 安装 `integ-feedback.service` 到 `/etc/systemd/system/`。
5. 安装 `discuss.caddy` 到 Caddy 的站点配置目录。
6. 将 `discuss.integ.life` 的 proxied A 记录指向生产服务器。
7. 为 `discuss.integ.life/sdk/*` 绑定 Cloudflare Worker；Worker 从
   `https://flyfy1.github.io/integ-feedback/sdk/*` 获取并缓存静态资源。
8. 运行 migration，启动服务，并验证：

```bash
systemctl enable --now integ-feedback
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
systemctl reload caddy
curl --fail https://discuss.integ.life/healthz
curl --fail https://discuss.integ.life/sdk/v1/comments.js
```

`ALLOWED_ORIGINS` 必须列出每个实际嵌入 SDK 的主站或子站 origin，不能使用 `*`。
