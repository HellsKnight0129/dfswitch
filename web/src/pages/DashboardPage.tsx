import { useState, useEffect, useMemo, useRef } from 'react'
import { api } from '../api/client'
import type { ApiKey, ToolInfo, ApplyResult } from '../api/client'

interface Props {
  onLogout: () => void
}

/* Inline SVG icons */
const IconKey = () => (
  <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="m21 2-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777Zm0 0L15.5 7.5m0 0 3 3L22 7l-3-3m-3.5 3.5L19 4" />
  </svg>
)
const IconTool = () => (
  <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76Z" />
  </svg>
)
const IconSync = () => (
  <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M21 12a9 9 0 0 0-9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" /><path d="M3 3v5h5" />
    <path d="M3 12a9 9 0 0 0 9 9 9.75 9.75 0 0 0 6.74-2.74L21 16" /><path d="M16 16h5v5" />
  </svg>
)

const platformLabel = (p: string) => {
  switch (p) {
    case 'anthropic': return 'Anthropic'
    case 'openai': return 'OpenAI'
    case 'gemini': return 'Gemini'
    case 'antigravity': return 'Antigravity'
    case 'minimax': return 'MiniMax'
    case 'custom': return 'Custom'
    default: return p || 'Unknown'
  }
}

const platformBadgeClass = (p: string) => {
  switch (p) {
    case 'anthropic': return 'plat-anthropic'
    case 'openai': return 'plat-openai'
    case 'gemini': return 'plat-gemini'
    case 'antigravity': return 'plat-antigravity'
    case 'minimax': return 'plat-minimax'
    case 'custom': return 'plat-custom'
    default: return 'plat-unknown'
  }
}

export default function DashboardPage({ onLogout }: Props) {
  const [keys, setKeys] = useState<ApiKey[]>([])
  const [tools, setTools] = useState<ToolInfo[]>([])
  const [selectedKeyId, setSelectedKeyId] = useState<number | null>(null)
  const [selectedTools, setSelectedTools] = useState<Set<string>>(new Set())
  const [results, setResults] = useState<ApplyResult[]>([])
  const [loading, setLoading] = useState(false)
  const [syncRunning, setSyncRunning] = useState(false)
  const [lastSync, setLastSync] = useState('')
  const [lastError, setLastError] = useState('')
  const [serverURL, setServerURL] = useState('')
  const [baseUrlOverride, setBaseUrlOverride] = useState('')
  const [baseUrlCustomized, setBaseUrlCustomized] = useState(false)

  useEffect(() => { loadData() }, [])

  /* Mouse parallax for orbs */
  const [parallax, setParallax] = useState({ x: 0, y: 0 })
  const [cursor, setCursor] = useState({ x: -1000, y: -1000 })
  const onMouseMove = (e: React.MouseEvent) => {
    setCursor({ x: e.clientX, y: e.clientY })
    setParallax({
      x: e.clientX / window.innerWidth - 0.5,
      y: e.clientY / window.innerHeight - 0.5,
    })
  }

  /* Embers */
  const embersRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const el = embersRef.current
    if (!el) return
    el.innerHTML = ''
    for (let i = 0; i < 40; i++) {
      const p = document.createElement('div')
      const size = 1.5 + Math.random() * 3
      const left = Math.random() * 100
      const dur = 14 + Math.random() * 18
      const delay = Math.random() * dur
      const opacity = 0.3 + Math.random() * 0.45
      const drift = (Math.random() - 0.5) * 80
      p.className = 'ember'
      p.style.cssText = `width:${size}px;height:${size}px;left:${left}%;bottom:-${size}px;animation-duration:${dur}s;animation-delay:-${delay}s;--ember-opacity:${opacity};--ember-drift:${drift}px;`
      el.appendChild(p)
    }
  }, [])

  const shaftStyle = (n: number): React.CSSProperties => {
    const left = 10 + ((n * 37) % 80)
    const delay = -n * 1.7
    const dur = 9 + ((n * 1.3) % 4)
    const hue = n % 2 === 0 ? '16,185,129' : '6,95,70'
    return {
      left: `${left}%`,
      animationDelay: `${delay}s`,
      animationDuration: `${dur}s`,
      background: `linear-gradient(to bottom, transparent, rgba(${hue},0.12), transparent)`,
    }
  }

  const loadData = async () => {
    try {
      const [keysResp, toolsResp, syncResp, settingsResp] = await Promise.all([
        api.getKeys(), api.getTools(), api.syncStatus(), api.getSettings(),
      ])
      setKeys(keysResp.items || [])
      setTools(toolsResp.tools || [])
      setSyncRunning(syncResp.running)
      setLastSync(syncResp.last_sync || '')
      setLastError(syncResp.last_error || '')
      setServerURL(settingsResp.server_url || '')
    } catch (err) {
      console.error('加载数据失败:', err)
    }
  }

  const selectedKey = useMemo(
    () => keys.find(k => k.id === selectedKeyId) ?? null,
    [keys, selectedKeyId],
  )

  const effectiveBaseURL = baseUrlCustomized
    ? baseUrlOverride
    : selectedKey?.suggested_base_url ?? ''

  const toggleTool = (id: string) => {
    // Silently ignore clicks on tools incompatible with the current key.
    const tool = tools.find(t => t.id === id)
    if (selectedKey && tool && tool.supported_platforms && !tool.supported_platforms.includes(selectedKey.platform)) {
      return
    }
    setSelectedTools(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id); else next.add(id)
      return next
    })
  }

  const selectKey = (k: ApiKey) => {
    setSelectedKeyId(k.id)
    setBaseUrlCustomized(false)
    setBaseUrlOverride(k.suggested_base_url)
    // Drop any previously-checked tools that don't support this key's platform.
    setSelectedTools(prev => {
      const next = new Set<string>()
      for (const id of prev) {
        const t = tools.find(tt => tt.id === id)
        if (t && (!t.supported_platforms || t.supported_platforms.includes(k.platform))) {
          next.add(id)
        }
      }
      return next
    })
  }

  const handleApply = async () => {
    if (!selectedKey || selectedTools.size === 0) return
    setLoading(true); setResults([])
    try {
      const resp = await api.apply({
        toolIds: Array.from(selectedTools),
        keyId: selectedKey.id,
        baseUrl: baseUrlCustomized ? baseUrlOverride : undefined,
      })
      setResults(resp.results)
    } catch (err: unknown) {
      const e = err as Error & { unsupported?: { tool_id: string; reason: string }[] }
      if (e.unsupported && e.unsupported.length > 0) {
        setResults(e.unsupported.map(u => ({ tool_id: u.tool_id, success: false, error: u.reason })))
      } else {
        setResults([{ tool_id: 'error', success: false, error: err instanceof Error ? err.message : '操作失败' }])
      }
    } finally { setLoading(false) }
  }

  const handleSync = async () => {
    try {
      if (syncRunning) { await api.syncStop(); setSyncRunning(false) }
      else { await api.syncStart(); setSyncRunning(true) }
    } catch (err) { console.error('同步操作失败:', err) }
  }

  const handleLogout = async () => { await api.logout(); onLogout() }

  const maskKey = (key: string) => key.length <= 12 ? key : `${key.slice(0, 8)}...${key.slice(-4)}`

  const statusBadge = (status: string) => {
    if (status === 'active') return 'badge-success'
    if (status === 'expired') return 'badge-danger'
    return 'badge-warning'
  }

  const installedCount = tools.filter(t => t.installed).length

  const formatUSD = (v: unknown) => {
    const n = typeof v === 'number' ? v : parseFloat(String(v ?? 0))
    if (!isFinite(n)) return '0'
    return n.toFixed(2)
  }

  return (
    <div className="nether-bg" onMouseMove={onMouseMove}>
      <div className="nether-layers">
        <div className="orb orb-a" style={{ transform: `translate3d(${parallax.x * -30}px, ${parallax.y * -30}px, 0)` }} />
        <div className="orb orb-b" style={{ transform: `translate3d(${parallax.x * 25}px, ${parallax.y * 25}px, 0)` }} />
        <div className="orb orb-c" />
        <div className="orb orb-d" />
        <div className="nether-grid" />
        <div className="sigil sigil-outer" />
        <div className="sigil sigil-mid" />
        <div className="sigil sigil-inner" />
      </div>
      <div className="light-shafts">
        {[1, 2, 3, 4, 5, 6].map(n => (
          <div key={n} className="light-shaft" style={shaftStyle(n)} />
        ))}
      </div>
      <div className="embers" ref={embersRef} />
      <div className="nether-vignette" />
      <div className="nether-scanline" />
      <div
        className="mouse-spotlight"
        style={{ background: `radial-gradient(600px circle at ${cursor.x}px ${cursor.y}px, rgba(16,185,129,0.07), transparent 65%)` }}
      />
      <header className="app-header">
        <div className="app-header-left">
          <div className="app-header-brand"><img src="/dflogo.png" alt="DF" /></div>
          <span className="app-header-title">DFSwitch</span>
        </div>

        {serverURL && <span className="app-header-center">{serverURL}</span>}

        <div className="app-header-right">
          <span className={`sync-dot ${syncRunning ? 'running' : 'stopped'}`} title={syncRunning ? '同步运行中' : '同步已停止'} />
          <button className="btn btn-ghost btn-sm" onClick={handleLogout}>退出</button>
        </div>
      </header>

      <div className="dashboard-content">

        <div className="stats-row">
          <div className="stat-card">
            <div className="stat-icon primary"><IconKey /></div>
            <div className="stat-info">
              <div className="stat-value">{keys.length}</div>
              <div className="stat-label">API Keys</div>
            </div>
          </div>
          <div className="stat-card">
            <div className="stat-icon success"><IconTool /></div>
            <div className="stat-info">
              <div className="stat-value">{installedCount}<span style={{ fontSize: '.9rem', fontWeight: 400, color: 'var(--slate-500)' }}>/{tools.length}</span></div>
              <div className="stat-label">已安装工具</div>
            </div>
          </div>
          <div className="stat-card">
            <div className="stat-icon warning"><IconSync /></div>
            <div className="stat-info">
              <div className="stat-value" style={{ fontSize: '1rem' }}>{syncRunning ? '运行中' : '已停止'}</div>
              <div className="stat-label">{lastSync ? `${new Date(lastSync).toLocaleString()}` : '自动同步'}</div>
            </div>
          </div>
        </div>

        {lastError && (
          <div className="error-banner" style={{ marginBottom: 12 }}>
            <span>同步错误: {lastError}</span>
          </div>
        )}

        <div className="card">
          <div className="card-header">
            <h3>API Keys</h3>
            <span className="badge badge-primary">{keys.length}</span>
          </div>
          <div className="card-body">
            <div className="key-list">
              {keys.map(k => {
                const isSelected = selectedKeyId === k.id
                const quota = typeof k.quota === 'number' ? k.quota : parseFloat(String(k.quota ?? 0))
                const used = typeof k.quota_used === 'number' ? k.quota_used : parseFloat(String(k.quota_used ?? 0))
                const pct = quota > 0 ? Math.min(100, (used / quota) * 100) : 0
                return (
                  <div key={k.id} className={`key-item ${isSelected ? 'selected' : ''}`} onClick={() => selectKey(k)}>
                    <input type="radio" checked={isSelected} onChange={() => selectKey(k)} />
                    <div className="key-info" style={{ flex: 1 }}>
                      <div className="key-name" style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
                        <span>{k.name || '未命名'}</span>
                        <span className={`badge ${platformBadgeClass(k.platform)}`}>{platformLabel(k.platform)}</span>
                        {k.group_name && <span className="key-group">{k.group_name}</span>}
                        {k.subscription_type && (
                          <span className="badge badge-muted">{k.subscription_type === 'subscription' ? '订阅' : '标准'}</span>
                        )}
                      </div>
                      <div className="key-meta">{maskKey(k.key)}</div>
                      {isSelected && (
                        <div className="key-details">
                          {quota > 0 && (
                            <div className="key-detail-row">
                              <span>额度: ${formatUSD(used)} / ${formatUSD(quota)}</span>
                              <div className="progress-bar"><div className="progress-fill" style={{ width: `${pct}%` }} /></div>
                            </div>
                          )}
                          <div className="key-detail-row">
                            <span>并发: {k.concurrency}</span>
                            {typeof k.rate_limit_1d === 'number' && k.rate_limit_1d > 0 && (
                              <span>日额度: ${formatUSD(k.usage_1d)} / ${formatUSD(k.rate_limit_1d)}</span>
                            )}
                          </div>
                          <div className="key-detail-row">
                            <label style={{ flex: 1 }}>
                              <span style={{ display: 'block', fontSize: '.72rem', color: 'var(--slate-500)', marginBottom: 3 }}>Gateway URL</span>
                              <input
                                type="text"
                                value={baseUrlCustomized ? baseUrlOverride : k.suggested_base_url}
                                readOnly={!baseUrlCustomized}
                                onChange={e => setBaseUrlOverride(e.target.value)}
                                onClick={e => e.stopPropagation()}
                                style={{ width: '100%' }}
                              />
                            </label>
                            <button
                              className="btn btn-ghost btn-sm"
                              onClick={e => {
                                e.stopPropagation()
                                setBaseUrlCustomized(v => !v)
                              }}
                            >
                              {baseUrlCustomized ? '恢复默认' : '自定义'}
                            </button>
                          </div>
                        </div>
                      )}
                    </div>
                    <div className="key-right">
                      <span className={`badge ${statusBadge(k.status)}`}>{k.status}</span>
                      {k.expires_at && (
                        <span className="key-expire">
                          {new Date(k.expires_at) < new Date() ? '已过期' : `${new Date(k.expires_at).toLocaleDateString()} 到期`}
                        </span>
                      )}
                    </div>
                  </div>
                )
              })}
              {keys.length === 0 && <div className="empty-state">暂无 API Key，请先在 Sub2API 中创建</div>}
            </div>
          </div>
        </div>

        <div className="card">
          <div className="card-header">
            <h3>目标工具</h3>
            {selectedTools.size > 0 && <span className="badge badge-primary">已选 {selectedTools.size}</span>}
          </div>
          <div className="card-body">
            <div className="tool-list">
              {tools.map(t => {
                const incompatible = selectedKey
                  ? !!(t.supported_platforms && !t.supported_platforms.includes(selectedKey.platform))
                  : false
                const tooltip = incompatible
                  ? `该工具不支持 ${platformLabel(selectedKey!.platform)} 平台 Key`
                  : t.config_path
                return (
                  <div
                    key={t.id}
                    className={`tool-item ${selectedTools.has(t.id) ? 'selected' : ''} ${!t.installed ? 'not-installed' : ''} ${incompatible ? 'incompatible' : ''}`}
                    onClick={() => !incompatible && toggleTool(t.id)}
                    title={tooltip}
                    style={incompatible ? { opacity: 0.4, cursor: 'not-allowed' } : undefined}
                  >
                    <input type="checkbox" checked={selectedTools.has(t.id)} disabled={incompatible} onChange={() => toggleTool(t.id)} />
                    <span className={`installed-dot ${t.installed ? 'yes' : 'no'}`} />
                    <div>
                      <div className="tool-name">{t.name}</div>
                      <div className="tool-desc">{t.description}</div>
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        </div>

        <div className="actions-bar">
          <button className="btn btn-primary" onClick={handleApply} disabled={loading || !selectedKey || selectedTools.size === 0}>
            {loading ? (<><span className="spinner" />写入中...</>) : '一键导入'}
          </button>
          <button className={`btn ${syncRunning ? 'btn-danger' : 'btn-secondary'}`} onClick={handleSync}>
            {syncRunning ? '停止同步' : '开启自动同步'}
          </button>
          {effectiveBaseURL && <span className="sync-info" style={{ fontFamily: 'monospace' }}>{effectiveBaseURL}</span>}
          {lastSync && <span className="sync-info">上次同步: {new Date(lastSync).toLocaleString()}</span>}
        </div>

        {results.length > 0 && (
          <div className="card">
            <div className="card-header">
              <h3>执行结果</h3>
            </div>
            <div className="card-body">
              {results.map((r, i) => (
                <div key={i} className={`result-item ${r.success ? 'success' : 'fail'}`}>
                  <div>{r.success ? '✓' : '✗'} {r.tool_id} {r.error ? `— ${r.error}` : ''}</div>
                  {r.warnings && r.warnings.length > 0 && (
                    <ul style={{ margin: '6px 0 0 20px', padding: 0, fontSize: '.8rem', color: 'var(--warning-600)' }}>
                      {r.warnings.map((w, j) => (
                        <li key={j}>⚠️ {w}</li>
                      ))}
                    </ul>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
