import { useState, useEffect, useRef } from 'react'
import { api } from '../api/client'

interface Props {
  onLogin: () => void
}

/* Inline SVG icons */
const IconServer = () => (
  <svg className="input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <rect x="2" y="2" width="20" height="8" rx="2" /><rect x="2" y="14" width="20" height="8" rx="2" />
    <line x1="6" y1="6" x2="6.01" y2="6" /><line x1="6" y1="18" x2="6.01" y2="18" />
  </svg>
)
const IconMail = () => (
  <svg className="input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <rect x="2" y="4" width="20" height="16" rx="2" /><path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7" />
  </svg>
)
const IconLock = () => (
  <svg className="input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <rect x="3" y="11" width="18" height="11" rx="2" /><path d="M7 11V7a5 5 0 0 1 10 0v4" />
  </svg>
)
const IconShield = () => (
  <svg className="input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10Z" />
  </svg>
)
const IconEye = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M2 12s3-7 10-7 10 7 10 7-3 7-10 7-10-7-10-7Z" /><circle cx="12" cy="12" r="3" />
  </svg>
)
const IconEyeOff = () => (
  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M9.88 9.88a3 3 0 1 0 4.24 4.24" /><path d="M10.73 5.08A10.43 10.43 0 0 1 12 5c7 0 10 7 10 7a13.16 13.16 0 0 1-1.67 2.68" />
    <path d="M6.61 6.61A13.526 13.526 0 0 0 2 12s3 7 10 7a9.74 9.74 0 0 0 5.39-1.61" /><line x1="2" y1="2" x2="22" y2="22" />
  </svg>
)
const IconAlert = () => (
  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="10" /><line x1="12" y1="8" x2="12" y2="12" /><line x1="12" y1="16" x2="12.01" y2="16" />
  </svg>
)

type Stage =
  | { kind: 'password' }
  | { kind: '2fa'; tempToken: string; emailMasked: string }

const TYPING_LINES = [
  'curl -X POST https://gate/v1/messages',
  '魂票已签 · 接引司已放行',
  '200 OK · tokens=8,192',
  '三途路通 · 百模皆渡 → DFSwitch',
]

export default function LoginPage({ onLogin }: Props) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [serverUrl, setServerUrl] = useState('https://df.dawnloadai.com:8443')
  const [showPwd, setShowPwd] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [stage, setStage] = useState<Stage>({ kind: 'password' })
  const [totpCode, setTotpCode] = useState('')

  /* Mouse parallax + spotlight */
  const [cursor, setCursor] = useState({ x: -1000, y: -1000 })
  const [parallax, setParallax] = useState({ x: 0, y: 0 })
  const onMouseMove = (e: React.MouseEvent) => {
    setCursor({ x: e.clientX, y: e.clientY })
    setParallax({
      x: e.clientX / window.innerWidth - 0.5,
      y: e.clientY / window.innerHeight - 0.5,
    })
  }

  /* Ember particles */
  const embersRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const el = embersRef.current
    if (!el) return
    el.innerHTML = ''
    for (let i = 0; i < 45; i++) {
      const p = document.createElement('div')
      const size = 1.5 + Math.random() * 3
      const left = Math.random() * 100
      const dur = 12 + Math.random() * 20
      const delay = Math.random() * dur
      const opacity = 0.35 + Math.random() * 0.5
      const drift = (Math.random() - 0.5) * 80
      p.className = 'ember'
      p.style.cssText = `width:${size}px;height:${size}px;left:${left}%;bottom:-${size}px;animation-duration:${dur}s;animation-delay:-${delay}s;--ember-opacity:${opacity};--ember-drift:${drift}px;`
      el.appendChild(p)
    }
  }, [])

  /* Typing effect */
  const [typed, setTyped] = useState('')
  useEffect(() => {
    let idx = 0
    let charIdx = 0
    let pause = 0
    let alive = true
    const tick = () => {
      if (!alive) return
      const line = TYPING_LINES[idx % TYPING_LINES.length]
      if (pause > 0) { pause--; setTyped(t => t) }
      else if (charIdx < line.length) {
        charIdx++
        setTyped(line.slice(0, charIdx))
        if (charIdx === line.length) pause = 50
      } else {
        setTyped(prev => {
          if (prev.length > 0) return prev.slice(0, -1)
          charIdx = 0
          idx++
          return ''
        })
      }
    }
    const id = window.setInterval(tick, 55)
    return () => { alive = false; window.clearInterval(id) }
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const result = await api.login(email, password, serverUrl)
      if ('requires_2fa' in result && result.requires_2fa) {
        setStage({ kind: '2fa', tempToken: result.temp_token, emailMasked: result.user_email_masked })
        setTotpCode('')
      } else {
        onLogin()
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '登录失败')
    } finally {
      setLoading(false)
    }
  }

  const handle2FASubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (stage.kind !== '2fa') return
    setError('')
    setLoading(true)
    try {
      await api.login2fa(stage.tempToken, totpCode)
      onLogin()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '验证失败')
    } finally {
      setLoading(false)
    }
  }

  const backToPassword = () => {
    setStage({ kind: 'password' })
    setError('')
    setTotpCode('')
  }

  const shaftStyle = (n: number): React.CSSProperties => {
    const left = 10 + ((n * 37) % 80)
    const delay = -n * 1.7
    const dur = 8 + ((n * 1.3) % 4)
    const hue = n % 2 === 0 ? '16,185,129' : '6,95,70'
    return {
      left: `${left}%`,
      animationDelay: `${delay}s`,
      animationDuration: `${dur}s`,
      background: `linear-gradient(to bottom, transparent, rgba(${hue},0.14), transparent)`,
    }
  }

  return (
    <div className="login-page" onMouseMove={onMouseMove}>
      {/* Orb parallax */}
      <div className="nether-layers">
        <div className="orb orb-a" style={{ transform: `translate3d(${parallax.x * -30}px, ${parallax.y * -30}px, 0)` }} />
        <div className="orb orb-b" style={{ transform: `translate3d(${parallax.x * 25}px, ${parallax.y * 25}px, 0)` }} />
        <div className="orb orb-c" />
        <div className="orb orb-d" />
      </div>

      {/* Grid */}
      <div className="login-grid-overlay" />

      {/* Six light shafts */}
      <div className="light-shafts">
        {[1, 2, 3, 4, 5, 6].map(n => (
          <div key={n} className="light-shaft" style={shaftStyle(n)} />
        ))}
      </div>

      {/* Sigil rings */}
      <div className="sigil sigil-outer" />
      <div className="sigil sigil-mid" />
      <div className="sigil sigil-inner" />

      {/* Embers */}
      <div className="embers" ref={embersRef} />

      {/* Vignette + scanline */}
      <div className="nether-vignette" />
      <div className="nether-scanline" />

      {/* Mouse spotlight */}
      <div
        className="mouse-spotlight"
        style={{
          background: `radial-gradient(480px circle at ${cursor.x}px ${cursor.y}px, rgba(16,185,129,0.09), transparent 65%)`,
        }}
      />

      {/* Pulse rings behind card */}
      <div className="pulse-ring" />
      <div className="pulse-ring" style={{ animationDelay: '-3s' }} />
      <div className="pulse-ring" style={{ animationDelay: '-6s' }} />

      <div className="login-container">
        <div className="login-brand">
          <div className="login-brand-icon">
            <img src="/dflogo.png" alt="DFSwitch" />
          </div>

          <div className="login-hero-tag">
            <span className="dot-ping" />
            <span>DFSWITCH · 接引司</span>
            <span className="sep">|</span>
            <span className="ver">v1.0</span>
          </div>

          <h1>
            <span className="glitch-wrap">
              <span className="glitch-main">DFSwitch</span>
              <span className="glitch-copy glitch-copy-1" aria-hidden="true">DFSwitch</span>
              <span className="glitch-copy glitch-copy-2" aria-hidden="true">DFSwitch</span>
            </span>
          </h1>
          <p>AI 工具 · 接引配置</p>

          <div className="type-line brand-typed">
            <span className="type-prompt">$</span>
            <span>{typed}</span>
            <span className="type-cursor" />
          </div>
        </div>

        <div className="card-glass">
          <div className="login-card">
            {stage.kind === 'password' ? (
              <>
                <div className="login-card-title">欢迎回来</div>
                <div className="login-card-subtitle">登录你的 Sub2API 账号</div>

                {error && (
                  <div className="error-banner">
                    <IconAlert />
                    <span>{error}</span>
                  </div>
                )}

                <form onSubmit={handleSubmit}>
                  <div className="input-group">
                    <label className="input-label">服务器地址</label>
                    <div className="input-wrapper">
                      <IconServer />
                      <input className="input" type="url" value={serverUrl} onChange={e => setServerUrl(e.target.value)} placeholder="https://your-server.com" />
                    </div>
                  </div>

                  <div className="input-group">
                    <label className="input-label">邮箱</label>
                    <div className="input-wrapper">
                      <IconMail />
                      <input className="input" type="email" value={email} onChange={e => setEmail(e.target.value)} placeholder="your@email.com" required />
                    </div>
                  </div>

                  <div className="input-group">
                    <label className="input-label">密码</label>
                    <div className="input-wrapper">
                      <IconLock />
                      <input className="input" type={showPwd ? 'text' : 'password'} value={password} onChange={e => setPassword(e.target.value)} placeholder="输入密码" required />
                      <button type="button" className="input-action" onClick={() => setShowPwd(v => !v)} tabIndex={-1}>
                        {showPwd ? <IconEyeOff /> : <IconEye />}
                      </button>
                    </div>
                  </div>

                  <button type="submit" className="btn btn-primary btn-full" disabled={loading}>
                    {loading ? (<><span className="spinner" />接引中...</>) : '开坛登录'}
                  </button>
                </form>
              </>
            ) : (
              <>
                <div className="login-card-title">两步验证</div>
                <div className="login-card-subtitle">账号 {stage.emailMasked} 已开启两步验证，请输入动态码</div>

                {error && (
                  <div className="error-banner">
                    <IconAlert />
                    <span>{error}</span>
                  </div>
                )}

                <form onSubmit={handle2FASubmit}>
                  <div className="input-group">
                    <label className="input-label">6 位验证码</label>
                    <div className="input-wrapper">
                      <IconShield />
                      <input
                        className="input"
                        type="text"
                        inputMode="numeric"
                        autoFocus
                        pattern="\d{6}"
                        maxLength={6}
                        value={totpCode}
                        onChange={e => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                        placeholder="123456"
                        required
                      />
                    </div>
                  </div>

                  <button type="submit" className="btn btn-primary btn-full" disabled={loading || totpCode.length !== 6}>
                    {loading ? (<><span className="spinner" />验证中...</>) : '验证并登录'}
                  </button>
                  <button type="button" className="btn btn-ghost btn-full" style={{ marginTop: 8 }} onClick={backToPassword}>
                    返回
                  </button>
                </form>
              </>
            )}
          </div>
        </div>

        <div className="login-footer">&copy; 2026 DFSWITCH · NETHERWORLD GATE</div>
      </div>
    </div>
  )
}
