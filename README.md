# Bookkeeping API

一个使用 Go + MySQL 实现的订单记录系统，包含 API 服务和内置 HTML5 Web 页面。

## 功能

- 订单创建、查询、修改、软删除
- 创建订单支持保存回调请求 URL
- 后台修改订单状态时，会向订单的回调 URL 推送状态变更
- 创建和修改订单时，手机号必须包含国家电话区号，金额必须大于等于 10
- 创建订单时 Telegram 用户 ID 可为空；为空时修改通知状态不会发送用户通知
- 创建订单时订单状态和通知状态可为空，默认都是 `pending`
- 修改订单时订单状态和通知状态不能为空
- MySQL 持久化存储
- `.env` 配置加载
- 首页 30 天订单金额趋势统计
- 订单记录管理页面
- 按订单状态统计每日金额
- Telegram 机器人通知

## 页面

启动后访问：

```text
http://127.0.0.1:5555/        统计首页
http://127.0.0.1:5555/orders  订单记录
http://127.0.0.1:5555/api-test 接口测试，可手动填写 X-API-Key
```

如果 `.env` 中 `API_TEST_ENABLED=false`，`/api-test` 页面不会提供，导航入口也会隐藏。

## 配置

复制或修改项目根目录的 `.env`：

```env
ADDR=:5555
API_KEY=change_me
WEB_USERNAME=admin
WEB_PASSWORD=change_me
SESSION_SECRET=change_me_to_a_long_random_secret
API_TEST_ENABLED=true
MYSQL_DSN=user:password@tcp(127.0.0.1:3306)/bookkeeping?parseTime=true&loc=UTC
TELEGRAM_BOT_TOKEN=your_bot_token
TELEGRAM_ADMIN_CHAT_ID=123456789
TELEGRAM_API_BASE=https://api.telegram.org
```

说明：

- `ADDR`: 服务监听地址，默认 `:5555`
- `API_KEY`: API 鉴权密钥，请部署时改成随机强密钥
- `WEB_USERNAME`: 前台登录用户名
- `WEB_PASSWORD`: 前台登录密码
- `SESSION_SECRET`: 登录 Cookie 签名密钥，请部署时改成随机强密钥
- `API_TEST_ENABLED`: 是否启用接口测试页面，设置为 `false` 时不提供 `/api-test`
- `MYSQL_DSN`: MySQL 连接串，必须包含 `parseTime=true`
- `TELEGRAM_BOT_TOKEN`: Telegram Bot Token
- `TELEGRAM_ADMIN_CHAT_ID`: 管理员 Telegram ID，创建订单成功后会通知该账号
- `TELEGRAM_API_BASE`: Telegram API 地址，默认 `https://api.telegram.org`

## 数据库

先创建数据库：

```sql
CREATE DATABASE IF NOT EXISTS bookkeeping
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_unicode_ci;
```

服务启动时会自动创建 `bookkeeping_orders` 表。建表 SQL 保存在：

```text
migrations/001_create_bookkeeping_orders.sql
```

## 启动

```bash
go run ./cmd/server
```

## API 文档

完整接口文档见：

```text
API.md
```

主要接口：

- `GET /healthz`
- `POST /api/v1/orders`
- `GET /api/v1/orders`
- `GET /api/v1/orders/lookup`
- `GET /api/v1/orders/daily_totals`
- `GET /api/v1/orders/daily_status_totals`

所有 `/api/` 接口都需要鉴权。外部调用请求头二选一：

```http
X-API-Key: your_api_key
Authorization: Bearer your_api_key
```

内置 HTML5 页面使用 `/login` 登录页，登录成功后通过 Cookie 调用后台管理接口。外部 API Key 不开放修改订单和删除订单。

## Telegram 通知

创建订单成功后，如果配置了 `TELEGRAM_ADMIN_CHAT_ID`，服务会通过 Telegram 机器人通知管理员。

当订单修改为：

```json
{
  "notify_status": "sent"
}
```

并且修改前不是 `sent`、订单里的 `telegram_user_id` 大于 0 时，服务会调用 Telegram Bot API 发送订单通知。没有 Telegram 用户 ID 的订单不会发送用户通知，重复保存 `sent` 不会重复发送。

## 登录验证码

前台登录页会显示一个简单数学验证码。验证码有效期 5 分钟，用户名、密码、验证码都正确后才会进入首页或订单记录页面。

## 测试

```bash
go test ./...
```
