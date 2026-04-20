# sub2api Backend 接口真实响应快照（Phase 0）

**探查时间**：2026-04-19
**目标**：`https://df.dawnloadai.com:8443`
**测试账号**：mxq@qq.com（role: user，allowed_groups: [3]，status: active）

---

## 响应外壳

所有用户接口统一：
```json
{"code": 0, "message": "success", "data": <payload>}
```

## 1. POST /api/v1/auth/login

**Request**: `{"email":"...","password":"..."}`

**200 Response**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "rt_...",
    "expires_in": 86400,
    "token_type": "Bearer",
    "user": {
      "id": 7,
      "email": "mxq@qq.com",
      "username": "",
      "role": "user",
      "balance": 0,
      "concurrency": 5,
      "status": "active",
      "allowed_groups": [3],
      "created_at": "...",
      "updated_at": "..."
    }
  }
}
```

## 2. POST /api/v1/auth/login/2fa

**Request**: `{"temp_token":"...","totp_code":"123456"}`
**Response**: 同 login 200（未实测，账号未开 2FA）

## 3. POST /api/v1/auth/refresh

**Request**: `{"refresh_token":"rt_..."}` （JSON body，不是 header！）

**200 Response**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJ...",
    "refresh_token": "rt_...",
    "expires_in": 86400,
    "token_type": "Bearer"
  }
}
```
✅ refresh_token 确认轮换；access + refresh 都更新。

## 4. GET /api/v1/keys?page=1&page_size=100

**200 Response** (envelope: `{code, message, data: {items, total, page, page_size, pages}}`)：

每个 item 字段：
```json
{
  "id": 177,
  "user_id": 7,
  "key": "sk-...（明文，敏感！）",
  "name": "购买-MiniMax-M2.7-highspeed-周卡",
  "group_id": 3,
  "status": "active",
  "ip_whitelist": null,
  "ip_blacklist": null,
  "last_used_at": null,
  "quota": 0,
  "quota_used": 0,
  "expires_at": "...",
  "created_at": "...",
  "updated_at": "...",
  "rate_limit_5h": 0,
  "rate_limit_1d": 0,
  "rate_limit_7d": 0,
  "usage_5h": 0,
  "usage_1d": 0,
  "usage_7d": 0,
  "window_5h_start": null,
  "window_1d_start": null,
  "window_7d_start": null,
  "concurrency": 5,
  "group": {
    "id": 3,
    "name": "MiniMax-M2.7-highspeed",
    "description": "",
    "platform": "minimax",
    "rate_multiplier": 1,
    "is_exclusive": true,
    "status": "active",
    "subscription_type": "subscription",
    "daily_limit_usd": 0,
    "weekly_limit_usd": 0,
    "monthly_limit_usd": 0,
    "claude_code_only": false,
    "allow_messages_dispatch": false,
    "require_oauth_only": false,
    "require_privacy_set": false,
    "created_at": "...",
    "updated_at": "..."
  },
  "subscription": {
    "id": 9,
    "user_id": 7,
    "group_id": 3,
    "starts_at": "...",
    "expires_at": "...",
    "status": "active",
    "daily_usage_usd": 0,
    "weekly_usage_usd": 0,
    "monthly_usage_usd": 0,
    "group": { /* 同上 */ }
  }
}
```

### Platform 值域（含实测）
从 [domain/constants.go:20-27](backend/internal/domain/constants.go)：
- `anthropic` → 网关 `{host}/v1`（Anthropic Messages API）
- `openai` → 网关 `{host}/v1`（OpenAI Chat Completions）
- `gemini` → 网关 `{host}/v1beta`（Gemini API）
- `antigravity` → 网关 `{host}/antigravity/v1`
- `minimax` → 网关 `{host}/v1`（Anthropic 兼容，`/v1/messages`）— **实测账号里就是这个**
- `custom` → 网关 `{host}/v1`（同 anthropic 兼容路径）

Status 值域（domain/constants.go:4-11）：`active` / `disabled` / `error` / `unused` / `used` / `expired`。实测看到 `active`。

Subscription_type：`standard` / `subscription`。

## 5. GET /api/v1/auth/me

**200 Response**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": 7,
    "email": "mxq@qq.com",
    "username": "",
    "role": "user",
    "balance": 0,
    "concurrency": 5,
    "status": "active",
    "allowed_groups": [3],
    "created_at": "...",
    "updated_at": "...",
    "run_mode": "standard"
  }
}
```
✅ `run_mode` 是 flat（和其他 User 字段同层），不嵌套。

## 6. 中间件 401 shape（字符串 code）

**Invalid Bearer**:
```json
{"code":"INVALID_TOKEN","message":"Invalid token"}
```

**No auth header**:
```json
{"code":"UNAUTHORIZED","message":"Authorization header is required"}
```

其他可能的 code：`TOKEN_EXPIRED`、`EMPTY_TOKEN`、`TOKEN_REVOKED`、`USER_NOT_FOUND`、`USER_INACTIVE`、`INVALID_AUTH_HEADER`。

**Refresh 触发条件**：HTTP 401 + code ∈ { `TOKEN_EXPIRED`, `INVALID_TOKEN`, `TOKEN_REVOKED` }。
对 `UNAUTHORIZED` / `USER_NOT_FOUND` / `USER_INACTIVE` 不 refresh（refresh 也救不回来），直接转登录。

## 关键修正（vs 最初 plan）

1. **Platform 值域新增 `minimax` 和 `custom`**：两者网关路径都是 `{host}/v1`。`GatewayURL` 需处理这两个值。
2. **Sync 判断 key status** 用常量 `"active"`（✅ 和 sync.go 当前写死一致，无需改）。
3. **Refresh 401 判断**：用字符串 code 识别，而不是裸 HTTP 401（避免业务 401 被误当作过期）。
