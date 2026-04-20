import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { useState, useEffect } from 'react'
import { api } from './api/client'
import LoginPage from './pages/LoginPage'
import DashboardPage from './pages/DashboardPage'
import './App.css'

function App() {
  const [loggedIn, setLoggedIn] = useState<boolean | null>(null)

  useEffect(() => {
    api.authStatus()
      .then(s => setLoggedIn(s.logged_in))
      .catch(() => setLoggedIn(false))
  }, [])

  if (loggedIn === null) {
    return <div className="loading">加载中...</div>
  }

  return (
    <BrowserRouter>
      <div className="wails-titlebar" />
      <Routes>
        <Route path="/login" element={
          loggedIn ? <Navigate to="/" /> : <LoginPage onLogin={() => setLoggedIn(true)} />
        } />
        <Route path="/" element={
          loggedIn ? <DashboardPage onLogout={() => setLoggedIn(false)} /> : <Navigate to="/login" />
        } />
      </Routes>
    </BrowserRouter>
  )
}

export default App
