# maxClaw 官网部署指南

## 域名信息
- **域名**: maxclaw.top
- **托管**: Vercel
- **源码位置**: `/website/index.html`

---

## 部署方式

### 方式一：Vercel CLI（最快）

```bash
# 1. 安装 Vercel CLI
npm install -g vercel

# 2. 登录 Vercel
vercel login

# 3. 进入网站目录并部署
cd website
vercel --prod
```

部署完成后，Vercel 会给你一个 `.vercel.app` 的临时域名。

---

### 方式二：GitHub + Vercel 自动部署（推荐）

#### 步骤 1: 将代码推送到 GitHub

```bash
# 添加 website 目录到 git
git add website/
git commit -m "feat: add maxclaw website"
git push origin main
```

#### 步骤 2: Vercel 配置

1. 访问 [vercel.com](https://vercel.com) 并登录
2. 点击 "Add New Project"
3. 导入 `maxclaw` GitHub 仓库
4. 配置如下：
   - **Framework Preset**: `Other`
   - **Root Directory**: `website`
   - **Build Command**: 留空（静态 HTML）
   - **Output Directory**: `./`

5. 点击 Deploy

---

### 步骤 3: 绑定域名 maxclaw.top

#### 在 Vercel 端配置：

1. 进入项目 Dashboard
2. 点击 "Settings" → "Domains"
3. 输入 `maxclaw.top` 并添加
4. 选择 "Add maxclaw.top"

Vercel 会显示需要的 DNS 记录：
```
Type: A
Name: @
Value: 76.76.21.21
```

#### 在域名服务商处配置 DNS：

登录你的域名服务商（如阿里云、腾讯云、Namecheap 等），添加 DNS 解析：

| 记录类型 | 主机记录 | 记录值 |
|---------|---------|--------|
| A | @ | 76.76.21.21 |
| CNAME | www | cname.vercel-dns.com |

> 注意：76.76.21.21 是 Vercel 的 Anycast IP，以 Vercel 实际显示的为准

#### 等待 DNS 生效：

通常 5-30 分钟生效，可以通过以下命令检查：
```bash
nslookup maxclaw.top
```

---

## 可选：配置 www 重定向

在 Vercel Domains 设置中，可以配置：
- `www.maxclaw.top` 重定向到 `maxclaw.top`
- 或反之

---

## HTTPS 自动配置

Vercel 会自动为域名配置 SSL 证书（Let's Encrypt），无需手动操作。

---

## 后续更新

使用 GitHub 方式部署后，每次 push 到 main 分支会自动触发重新部署。

```bash
# 修改网站内容
git add website/
git commit -m "feat: update website content"
git push origin main
# 自动部署！
```

---

## 验证清单

- [ ] 网站可以通过 vercel.app 域名访问
- [ ] DNS 解析正确指向 Vercel
- [ ] maxclaw.top 可以正常访问
- [ ] HTTPS 证书生效
- [ ] www.maxclaw.top 重定向正确（如配置）
