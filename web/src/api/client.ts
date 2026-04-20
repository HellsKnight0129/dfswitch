// Wails binds every exported method on the Go App struct under
// window.go.main.App.<Method>. This module is a thin adapter that turns those
// bindings into the same shape the UI used when it was talking to a local Gin
// server, so DashboardPage / LoginPage didn't need to change.

type GoApp = {
  Login(email: string, password: string, serverURL: string): Promise<LoginResultRaw>
  Login2FA(tempToken: string, totpCode: string): Promise<LoginResultRaw>
  Logout(): Promise<void>
  AuthStatus(): Promise<AuthStatusRaw>
  ListKeys(): Promise<ApiKey[] | null>
  ListTools(): Promise<ToolInfo[] | null>
  Apply(req: {
    tool_ids: string[]
    key_id: number
    api_key: string
    base_url: string
    platform: string
  }): Promise<ApplyResponseRaw>
  SyncStart(): Promise<void>
  SyncStop(): Promise<void>
  SyncStatus(): Promise<SyncStatusRaw>
  GetSettings(): Promise<SettingsRaw>
  UpdateSettings(req: {
    server_url?: string
    sync_interval_minutes?: number
  }): Promise<void>
  CheckUpdate(): Promise<UpdateInfoRaw>
  ApplyUpdate(): Promise<void>
}

type LoginResultRaw = {
  ok: boolean
  requires_2fa: boolean
  temp_token?: string
  user_email_masked?: string
  user?: UserInfo
}

type AuthStatusRaw = {
  logged_in: boolean
  server_url: string
  user?: UserInfo
}

type ApplyResponseRaw = {
  results: ApplyResult[] | null
  successes: number
  failures: number
  unsupported?: UnsupportedTool[]
}

type SyncStatusRaw = {
  running: boolean
  last_sync: string
  last_error: string
  interval: number
}

type SettingsRaw = {
  server_url: string
  sync_enabled: boolean
  sync_interval: number
}

type UpdateInfoRaw = {
  available: boolean
  current_version: string
  latest_version: string
  release_notes?: string
  download_url?: string
}

// Resolve lazily — at module import time Wails may still be wiring bindings up.
function goApp(): GoApp {
  // @ts-expect-error — injected by Wails runtime
  const app = window.go?.main?.App as GoApp | undefined
  if (!app) throw new Error('Wails bindings 未就绪')
  return app
}

export type TFAChallenge = {
  requires_2fa: true
  temp_token: string
  user_email_masked: string
}

export type LoginOk = {
  ok: true
  user?: UserInfo
}

export type LoginResult = TFAChallenge | LoginOk

export type UserInfo = {
  id: number
  email: string
  username?: string
  role: string
  balance?: number
  concurrency?: number
  status?: string
  run_mode?: string
}

function normaliseLogin(r: LoginResultRaw): LoginResult {
  if (r.requires_2fa) {
    return {
      requires_2fa: true,
      temp_token: r.temp_token ?? '',
      user_email_masked: r.user_email_masked ?? '',
    }
  }
  return { ok: true, user: r.user }
}

export const api = {
  login: async (email: string, password: string, serverUrl: string): Promise<LoginResult> =>
    normaliseLogin(await goApp().Login(email, password, serverUrl)),

  login2fa: async (tempToken: string, totpCode: string): Promise<LoginOk> => {
    const r = await goApp().Login2FA(tempToken, totpCode)
    return { ok: true, user: r.user }
  },

  logout: () => goApp().Logout(),

  authStatus: () => goApp().AuthStatus(),

  getKeys: async (): Promise<{ items: ApiKey[] }> => {
    const items = (await goApp().ListKeys()) ?? []
    return { items }
  },

  getTools: async (): Promise<{ tools: ToolInfo[] }> => {
    const tools = (await goApp().ListTools()) ?? []
    return { tools }
  },

  apply: async (params: {
    toolIds: string[]
    keyId?: number
    apiKey?: string
    baseUrl?: string
    platform?: string
  }): Promise<{ results: ApplyResult[]; successes: number; failures: number }> => {
    const r = await goApp().Apply({
      tool_ids: params.toolIds,
      key_id: params.keyId ?? 0,
      api_key: params.apiKey ?? '',
      base_url: params.baseUrl ?? '',
      platform: params.platform ?? '',
    })
    // Platform-mismatch branch: backend returns no results, only an
    // Unsupported list. Surface it as an Error so the existing UI branch
    // (err.unsupported) still fires.
    if (r.unsupported && r.unsupported.length > 0) {
      const e = new Error('部分工具与所选 Key 平台不兼容') as Error & {
        unsupported?: UnsupportedTool[]
      }
      e.unsupported = r.unsupported
      throw e
    }
    return {
      results: r.results ?? [],
      successes: r.successes,
      failures: r.failures,
    }
  },

  syncStart: () => goApp().SyncStart(),
  syncStop: () => goApp().SyncStop(),
  syncStatus: () => goApp().SyncStatus(),

  getSettings: () => goApp().GetSettings(),
  updateSettings: (settings: { server_url?: string; sync_interval_minutes?: number }) =>
    goApp().UpdateSettings(settings),

  checkUpdate: () => goApp().CheckUpdate(),
  applyUpdate: () => goApp().ApplyUpdate(),
}

export type ApiKey = {
  id: number
  key: string
  name: string
  group_id: number
  group_name: string
  platform: string
  subscription_type: string
  status: string
  quota: number | string
  quota_used: number | string
  rate_limit_1d: number | string
  usage_1d: number | string
  concurrency: number
  expires_at?: string
  last_used_at?: string
  suggested_base_url: string
}

export type ToolInfo = {
  id: string
  name: string
  description: string
  installed: boolean
  config_path: string
  supported_platforms?: string[]
}

export type ApplyResult = {
  tool_id: string
  success: boolean
  error?: string
  warnings?: string[]
}

export type UnsupportedTool = {
  tool_id: string
  supported: string[]
  reason: string
}
