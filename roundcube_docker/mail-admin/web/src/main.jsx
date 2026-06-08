import React, { useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import './styles.css';

const emptyForm = { email: '', password: '', quota: '' };

function App() {
  const [session, setSession] = useState(null);
  const [login, setLogin] = useState({ email: '', password: '' });
  const [users, setUsers] = useState([]);
  const [stats, setStats] = useState({});
  const [form, setForm] = useState(emptyForm);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [drawer, setDrawer] = useState(null);

  useEffect(() => {
    api('/api/me').then(setSession).catch(() => setSession(null));
  }, []);

  useEffect(() => {
    if (session) loadUsers();
  }, [session]);

  async function loadUsers() {
    const data = await api('/api/users');
    setUsers(data.users || []);
    setStats(data.stats || {});
  }

  async function submitLogin(event) {
    event.preventDefault();
    setError('');
    setBusy(true);
    try {
      const data = await api('/api/login', { method: 'POST', body: login });
      setSession(data);
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  async function logout() {
    await api('/api/logout', { method: 'POST', csrf: session.csrfToken });
    setSession(null);
  }

  async function createUser(event) {
    event.preventDefault();
    await mutate('/api/users', { method: 'POST', body: form });
    setForm(emptyForm);
    setDrawer(null);
  }

  async function resetPassword(event) {
    event.preventDefault();
    await mutate(`/api/users/${encodeURIComponent(drawer.email)}/password`, {
      method: 'PATCH',
      body: { password: drawer.password }
    });
    setDrawer(null);
  }

  async function setQuota(event) {
    event.preventDefault();
    await mutate(`/api/users/${encodeURIComponent(drawer.email)}/quota`, {
      method: 'PUT',
      body: { quota: drawer.quota }
    });
    setDrawer(null);
  }

  async function removeQuota(user) {
    await mutate(`/api/users/${encodeURIComponent(user.email)}/quota`, { method: 'DELETE' });
  }

  async function deleteUser(event) {
    event.preventDefault();
    await mutate(`/api/users/${encodeURIComponent(drawer.email)}`, {
      method: 'DELETE',
      body: { confirmEmail: drawer.confirmEmail }
    });
    setDrawer(null);
  }

  async function mutate(path, options) {
    setError('');
    setBusy(true);
    try {
      await api(path, { ...options, csrf: session.csrfToken });
      await loadUsers();
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  if (!session) {
    return (
      <main className="login-shell">
        <section className="login-card">
          <p className="eyebrow">PNP Mail</p>
          <h1>Mail Admin</h1>
          <p className="muted">Manage mailboxes and quotas for this Docker mailserver.</p>
          <form onSubmit={submitLogin} className="stack">
            <input placeholder="Admin email" value={login.email} onChange={e => setLogin({ ...login, email: e.target.value })} />
            <input placeholder="Password" type="password" value={login.password} onChange={e => setLogin({ ...login, password: e.target.value })} />
            {error && <div className="error">{error}</div>}
            <button disabled={busy}>{busy ? 'Signing in...' : 'Sign in'}</button>
          </form>
        </section>
      </main>
    );
  }

  return (
    <main className="app-shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">Mail Control</p>
          <h1>Mailboxes</h1>
        </div>
        <button className="ghost" onClick={logout}>Logout</button>
      </header>

      <section className="stats-grid">
        <Stat label="Users" value={stats.totalUsers ?? users.length} />
        <Stat label="Quota enabled" value={stats.quotaUsers ?? 0} />
        <Stat label="Mailserver" value={stats.mailserverReady ? 'Online' : 'Offline'} />
      </section>

      {error && <div className="error">{error}</div>}

      <section className="panel">
        <div className="panel-head">
          <div>
            <h2>Accounts</h2>
            <p className="muted">Add users, reset passwords, and control mailbox sizes.</p>
          </div>
          <button onClick={() => setDrawer({ type: 'add' })}>Add user</button>
        </div>

        <div className="user-list">
          {users.map(user => (
            <article className="user-card" key={user.email}>
              <div>
                <h3>{user.email}</h3>
                <p className="muted">{user.used} used / {user.quotaSet ? user.quota : 'Unlimited'}</p>
                <div className="meter"><span style={{ width: `${Math.min(user.percentUsed, 100)}%` }} /></div>
              </div>
              <div className="actions">
                <button className="ghost" onClick={() => setDrawer({ type: 'quota', email: user.email, quota: user.quotaSet ? user.quota : '' })}>Quota</button>
                <button className="ghost" onClick={() => setDrawer({ type: 'password', email: user.email, password: '' })}>Password</button>
                {user.quotaSet && <button className="ghost" onClick={() => removeQuota(user)}>No quota</button>}
                <button className="danger" onClick={() => setDrawer({ type: 'delete', email: user.email, confirmEmail: '' })}>Delete</button>
              </div>
            </article>
          ))}
        </div>
      </section>

      {drawer && (
        <div className="drawer-backdrop" onClick={() => setDrawer(null)}>
          <section className="drawer" onClick={event => event.stopPropagation()}>
            <button className="close" onClick={() => setDrawer(null)}>Close</button>
            {drawer.type === 'add' && (
              <form onSubmit={createUser} className="stack">
                <h2>Add mailbox</h2>
                <input placeholder="user@itbsstudio.com" value={form.email} onChange={e => setForm({ ...form, email: e.target.value })} />
                <input placeholder="Password" type="password" value={form.password} onChange={e => setForm({ ...form, password: e.target.value })} />
                <input placeholder="Quota, e.g. 1G (optional)" value={form.quota} onChange={e => setForm({ ...form, quota: e.target.value })} />
                <button disabled={busy}>Create mailbox</button>
              </form>
            )}
            {drawer.type === 'password' && (
              <form onSubmit={resetPassword} className="stack">
                <h2>Reset password</h2>
                <p className="muted">{drawer.email}</p>
                <input placeholder="New password" type="password" value={drawer.password} onChange={e => setDrawer({ ...drawer, password: e.target.value })} />
                <button disabled={busy}>Save password</button>
              </form>
            )}
            {drawer.type === 'quota' && (
              <form onSubmit={setQuota} className="stack">
                <h2>Set quota</h2>
                <p className="muted">{drawer.email}</p>
                <input placeholder="500M, 1G, 5G, 0B" value={drawer.quota} onChange={e => setDrawer({ ...drawer, quota: e.target.value })} />
                <button disabled={busy}>Save quota</button>
              </form>
            )}
            {drawer.type === 'delete' && (
              <form onSubmit={deleteUser} className="stack">
                <h2>Delete mailbox</h2>
                <p className="muted">Type the full email to confirm: {drawer.email}</p>
                <input placeholder={drawer.email} value={drawer.confirmEmail} onChange={e => setDrawer({ ...drawer, confirmEmail: e.target.value })} />
                <button className="danger" disabled={busy}>Delete mailbox</button>
              </form>
            )}
          </section>
        </div>
      )}
    </main>
  );
}

function Stat({ label, value }) {
  return <article className="stat"><span>{label}</span><strong>{value}</strong></article>;
}

async function api(path, options = {}) {
  const response = await fetch(path, {
    method: options.method || 'GET',
    credentials: 'same-origin',
    headers: {
      ...(options.body ? { 'Content-Type': 'application/json' } : {}),
      ...(options.csrf ? { 'X-CSRF-Token': options.csrf } : {})
    },
    body: options.body ? JSON.stringify(options.body) : undefined
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error || 'Request failed');
  return data;
}

createRoot(document.getElementById('root')).render(<App />);
