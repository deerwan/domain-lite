
## Docker 部署

### 方式：`docker run`

```bash
docker run -d \
  --name domain-lite \
  -p 8080:8080 \
  -e JWT_SECRET=$(openssl rand -base64 32) \# 生成一个随机字符串
  -e ADMIN_USERNAME=myadmin \
  -e ADMIN_PASSWORD=mypassword \
  -v domain-lite-data:/data \
  --restart unless-stopped \
  mrdeer1997/domain-lite:latest
```

### 镜像标签说明

- `:latest` —— 稳定版，仅在某次 `v*` 发版时更新，**生产/部署请用这个**。
- `:dev` —— 滚动预览版，跟随 `main` 分支的最新提交，可能包含未发布、不稳定的改动。想提前体验新功能或排查问题时使用：
