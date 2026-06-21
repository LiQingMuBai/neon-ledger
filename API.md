# API 接口文档

基础地址：

```text
http://127.0.0.1:5555
```

登录后可以访问浏览器接口测试页：

```text
http://127.0.0.1:5555/api-test
```

该页面支持手动填写 `X-API-Key` 请求头。

响应格式均为 JSON，除 `DELETE` 成功返回 `204 No Content` 外。

## 鉴权

所有 `/api/` 接口都需要鉴权，`/healthz` 不需要。

HTML5 页面使用 `/login` 登录页，登录成功后通过 HttpOnly Cookie 调用 API。用户名和密码由 `.env` 中的 `WEB_USERNAME`、`WEB_PASSWORD` 配置。

登录页包含简单数学验证码，验证码通过 `/captcha` 获取，有效期 5 分钟。

外部程序调用 API 时，请求头二选一：

```http
X-API-Key: your_api_key
```

或：

```http
Authorization: Bearer your_api_key
```

未提供或错误时返回：

```http
401 Unauthorized
```

```json
{
  "error": "unauthorized"
}
```

## 通用枚举

订单状态 `status`：

- `pending`
- `paid`
- `failed`
- `closed`

通知状态 `notify_status`：

- `pending`
- `sent`
- `failed`

## 通用错误

```json
{
  "error": "错误信息"
}
```

常见状态码：

- `400`: 请求参数错误
- `401`: 未鉴权或 API Key 错误
- `404`: 订单不存在
- `405`: 请求方法不允许
- `502`: Telegram 通知发送失败

## 健康检查

```http
GET /healthz
```

成功响应：

```json
{
  "status": "ok"
}
```

## 创建订单

```http
POST /api/v1/orders
Content-Type: application/json
```

请求体：

```json
{
  "customer_order_no": "C202606210001",
  "telegram_user_id": 987654321,
  "amount": 3599,
  "phone": "+8613800138000",
  "callback_url": "https://example.com/callback",
  "status": "pending",
  "notify_status": "pending"
}
```

字段说明：

- `customer_order_no`: 客户生成的订单号，必填且唯一
- `telegram_user_id`: Telegram 用户 ID，可不传；不传或传 `0` 时不会发送用户通知
- `amount`: 订单金额，整数，必须大于等于 10
- `phone`: 手机号码，必填，必须包含国家电话区号，例如 `+8613800138000`
- `callback_url`: 回调请求 URL，可不传或传空字符串；传入非空值时必须是 `http://` 或 `https://` URL
- `status`: 订单状态，创建时可不传或传空，服务端默认 `pending`
- `notify_status`: 通知状态，创建时可不传或传空，服务端默认 `pending`

成功响应：`201 Created`

创建成功后，如果服务配置了 `TELEGRAM_ADMIN_CHAT_ID`，会通过 Telegram 机器人通知管理员。通知失败不会影响订单创建响应，失败原因会输出到终端日志。

```json
{
  "id": "8c01d14a9a5f4d2d97f19ef63a7ab001",
  "customer_order_no": "C202606210001",
  "platform_order_no": "BK20260621123045A1B2C3D4",
  "telegram_user_id": 987654321,
  "amount": 3599,
  "phone": "+8613800138000",
  "callback_url": "https://example.com/callback",
  "status": "pending",
  "notify_status": "pending",
  "created_at": "2026-06-21T12:30:45Z",
  "updated_at": "2026-06-21T12:30:45Z"
}
```

## 查询订单

```http
GET /api/v1/orders
```

查询参数：

- `customer_order_no`: 客户订单号
- `platform_order_no`: 平台订单号
- `telegram_user_id`: Telegram 用户 ID
- `phone`: 手机号码，支持模糊匹配
- `status`: 订单状态
- `notify_status`: 通知状态
- `start_time`: RFC3339 时间，按 `created_at` 过滤
- `end_time`: RFC3339 时间，按 `created_at` 过滤
- `include_deleted`: 是否包含软删除订单，`true` 表示包含
- `limit`: 分页大小，默认 `20`，最大 `100`
- `offset`: 分页偏移，默认 `0`

示例：

```http
GET /api/v1/orders?telegram_user_id=987654321&status=paid&limit=20&offset=0
```

成功响应：

```json
{
  "items": [
    {
      "id": "8c01d14a9a5f4d2d97f19ef63a7ab001",
      "customer_order_no": "C202606210001",
      "platform_order_no": "BK20260621123045A1B2C3D4",
      "telegram_user_id": 987654321,
      "amount": 3599,
      "phone": "+8613800138000",
      "callback_url": "https://example.com/callback",
      "status": "paid",
      "notify_status": "sent",
      "created_at": "2026-06-21T12:30:45Z",
      "updated_at": "2026-06-21T12:40:00Z"
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0
}
```

## 按订单号获取订单

```http
GET /api/v1/orders/lookup
```

查询参数二选一：

- `customer_order_no`: 客户订单号
- `platform_order_no`: 平台订单号

示例：

```http
GET /api/v1/orders/lookup?customer_order_no=C202606210001
GET /api/v1/orders/lookup?platform_order_no=BK20260621123045A1B2C3D4
```

成功响应：`200 OK`

```json
{
  "id": "8c01d14a9a5f4d2d97f19ef63a7ab001",
  "customer_order_no": "C202606210001",
  "platform_order_no": "BK20260621123045A1B2C3D4",
  "telegram_user_id": 987654321,
  "amount": 3599,
  "phone": "+8613800138000",
  "callback_url": "https://example.com/callback",
  "status": "paid",
  "notify_status": "sent",
  "created_at": "2026-06-21T12:30:45Z",
  "updated_at": "2026-06-21T12:40:00Z"
}
```

没有传订单号返回 `400 Bad Request`，查不到订单返回 `404 Not Found`。

## 订单状态回调

后台修改订单时，如果订单 `status` 发生变化，并且订单存在 `callback_url`，服务会向该 URL 发起回调请求。回调失败会输出到终端日志，不影响订单保存。

```http
POST {callback_url}
Content-Type: application/json
```

请求体：

```json
{
  "customer_order_no": "C202606210001",
  "platform_order_no": "BK20260621123045A1B2C3D4",
  "status": "paid",
  "phone": "+8613800138000"
}
```

## 每日已支付订单总额

```http
GET /api/v1/orders/daily_totals
```

该接口只统计 `status=paid` 的订单。

查询参数：

- `start_time`: RFC3339 时间，按 `created_at` 过滤
- `end_time`: RFC3339 时间，按 `created_at` 过滤
- `notify_status`: 通知状态
- `include_deleted`: 是否包含软删除订单，`true` 表示包含

示例：

```http
GET /api/v1/orders/daily_totals?start_time=2026-06-01T00:00:00Z&end_time=2026-06-21T23:59:59Z
```

成功响应：

```json
{
  "items": [
    {
      "date": "2026-06-21",
      "total_amount": 3599,
      "order_count": 1
    }
  ]
}
```

## 每日按订单状态统计金额

```http
GET /api/v1/orders/daily_status_totals
```

首页折线图使用该接口。它按 `date + status` 分组统计订单金额。

查询参数：

- `start_time`: RFC3339 时间，按 `created_at` 过滤
- `end_time`: RFC3339 时间，按 `created_at` 过滤
- `notify_status`: 通知状态
- `include_deleted`: 是否包含软删除订单，`true` 表示包含

示例：

```http
GET /api/v1/orders/daily_status_totals?start_time=2026-06-01T00:00:00Z&end_time=2026-06-21T23:59:59Z
```

成功响应：

```json
{
  "items": [
    {
      "date": "2026-06-21",
      "status": "paid",
      "total_amount": 3599,
      "order_count": 1
    },
    {
      "date": "2026-06-21",
      "status": "pending",
      "total_amount": 1200,
      "order_count": 2
    }
  ]
}
```
