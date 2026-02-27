package handlers

import "net/http"

const uiHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Nusantara Panel UI</title>
  <style>
    :root {
      --bg: #f1f5f9;
      --panel: #ffffff;
      --ink: #0f172a;
      --muted: #475569;
      --brand: #0d9488;
      --line: #dbe2ea;
      --danger: #b91c1c;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      background: linear-gradient(145deg, #ecfeff, #e2e8f0);
      color: var(--ink);
      font: 14px/1.45 system-ui, -apple-system, "Segoe UI", sans-serif;
    }
    .wrap { max-width: 1080px; margin: 24px auto; padding: 0 16px; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(320px, 1fr)); gap: 16px; }
    .card {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 14px;
      box-shadow: 0 8px 24px rgba(2, 6, 23, 0.06);
      padding: 14px;
    }
    h1 { margin: 0 0 8px; font-size: 24px; }
    h2 { margin: 0 0 10px; font-size: 16px; }
    p { margin: 0 0 10px; color: var(--muted); }
    label { display: block; margin: 8px 0 4px; color: var(--muted); font-size: 12px; }
    input, select, textarea {
      width: 100%;
      padding: 10px;
      border: 1px solid #cbd5e1;
      border-radius: 10px;
      background: #fff;
      color: var(--ink);
      font: inherit;
    }
    textarea { min-height: 92px; resize: vertical; }
    .row { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 10px; }
    button {
      border: 0;
      border-radius: 10px;
      padding: 9px 12px;
      background: var(--brand);
      color: #fff;
      cursor: pointer;
      font-weight: 600;
    }
    button.alt { background: #334155; }
    button.warn { background: var(--danger); }
    .mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
    .token { background: #f8fafc; border: 1px dashed #cbd5e1; border-radius: 10px; padding: 10px; word-break: break-all; }
    pre {
      margin: 0;
      background: #0f172a;
      color: #e2e8f0;
      border-radius: 12px;
      padding: 12px;
      min-height: 220px;
      overflow: auto;
    }
    .ok { color: #047857; }
    .bad { color: var(--danger); }
    .pill {
      display: inline-block;
      border-radius: 999px;
      padding: 4px 10px;
      font-size: 12px;
      font-weight: 700;
      margin-top: 4px;
      background: #e2e8f0;
      color: #334155;
    }
    .pill.ok { background: #dcfce7; color: #166534; }
    .pill.run { background: #dbeafe; color: #1d4ed8; }
    .pill.err { background: #fee2e2; color: #b91c1c; }
    .bar {
      margin-top: 10px;
      width: 100%;
      height: 10px;
      background: #e2e8f0;
      border-radius: 999px;
      overflow: hidden;
    }
    .bar > i {
      display: block;
      height: 100%;
      width: 0%;
      background: linear-gradient(90deg, #0d9488, #0369a1);
      transition: width .25s ease;
    }
    .table {
      margin-top: 8px;
      border: 1px solid var(--line);
      border-radius: 10px;
      overflow: hidden;
      background: #f8fafc;
    }
    .site-row {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      gap: 8px;
      padding: 10px;
      border-top: 1px solid var(--line);
    }
    .site-row:first-child { border-top: 0; }
    .site-main { min-width: 0; }
    .site-domain { font-weight: 700; }
    .site-meta { font-size: 12px; color: #475569; margin-top: 2px; word-break: break-word; }
    .badge {
      display: inline-block;
      border-radius: 999px;
      padding: 2px 8px;
      font-size: 11px;
      font-weight: 700;
      margin-top: 4px;
      background: #e2e8f0;
      color: #334155;
    }
    .badge.active { background: #dcfce7; color: #166534; }
    .badge.provisioning, .badge.deleting { background: #dbeafe; color: #1d4ed8; }
    .badge.failed { background: #fee2e2; color: #b91c1c; }
    .mini {
      border: 0;
      border-radius: 8px;
      padding: 6px 10px;
      font-size: 12px;
      font-weight: 700;
      background: #b91c1c;
      color: #fff;
      cursor: pointer;
      white-space: nowrap;
    }
    .muted { color: #64748b; font-size: 12px; }
  </style>
</head>
<body>
  <div class="wrap">
    <h1>Nusantara Panel UI</h1>
    <p>UI testing panel for API v1 (single-node preview).</p>

    <div class="grid">
      <section class="card">
        <h2>Auth</h2>
        <label>Username</label>
        <input id="username" value="admin" autocomplete="username">
        <label>Password</label>
        <input id="password" type="password" autocomplete="current-password">
        <div class="row">
          <button id="btnLogin">Login</button>
          <button class="alt" id="btnMe">Me</button>
          <button class="warn" id="btnLogout">Logout</button>
        </div>
        <label>Token</label>
        <div id="token" class="token mono">(empty)</div>
        <p id="authStatus"></p>
      </section>

      <section class="card">
        <h2>Quick Actions</h2>
        <div class="row">
          <button class="alt" id="btnHealth">GET /healthz</button>
          <button class="alt" id="btnCompat">GET /v1/system/compatibility</button>
          <button class="alt" id="btnSites">GET /v1/sites</button>
          <button class="alt" id="btnJobs">GET /v1/jobs</button>
        </div>
      </section>

      <section class="card">
        <h2>Panel Version</h2>
        <p id="versionMeta">Version not loaded.</p>
        <div class="row">
          <button class="alt" id="btnPanelVersion">GET /v1/panel/version</button>
        </div>
      </section>

      <section class="card">
        <h2>Panel Update</h2>
        <p>Run update without SSH (admin only).</p>
        <div id="updateState" class="pill">Unknown</div>
        <div class="bar"><i id="updateProgress"></i></div>
        <p id="updateMeta">Click status to load current updater state.</p>
        <div class="row">
          <button id="btnStartUpdate">POST /v1/panel/update</button>
          <button class="alt" id="btnUpdateStatus">GET /v1/panel/update/status</button>
        </div>
      </section>

      <section class="card">
        <h2>Create Site</h2>
        <label>Domain</label>
        <input id="domain" placeholder="example.com">
        <label>Root Path</label>
        <input id="rootPath" value="/var/www/example.com/public">
        <label>Runtime</label>
        <select id="runtime">
          <option value="php">php</option>
          <option value="node">node</option>
          <option value="python">python</option>
          <option value="static">static</option>
        </select>
        <div class="row">
          <button id="btnCreateSite">POST /v1/sites</button>
        </div>
      </section>

      <section class="card">
        <h2>Sites Explorer</h2>
        <p id="sitesMeta">Login to load sites list.</p>
        <div class="row">
          <button class="alt" id="btnSitesRefresh">Refresh Sites</button>
        </div>
        <div id="sitesList" class="table">
          <div class="site-row"><div class="muted">No data loaded.</div></div>
        </div>
      </section>
    </div>

    <section class="card" style="margin-top:16px">
      <h2>Response</h2>
      <pre id="out" class="mono">Ready.</pre>
    </section>
  </div>

  <script>
    (function () {
      var out = document.getElementById('out');
      var tokenBox = document.getElementById('token');
      var authStatus = document.getElementById('authStatus');
      var versionMeta = document.getElementById('versionMeta');
      var sitesMeta = document.getElementById('sitesMeta');
      var sitesList = document.getElementById('sitesList');
      var updateState = document.getElementById('updateState');
      var updateMeta = document.getElementById('updateMeta');
      var updateProgress = document.getElementById('updateProgress');
      var btnStartUpdate = document.getElementById('btnStartUpdate');
      var token = localStorage.getItem('nusantara_token') || '';
      var updatePollTimer = null;
      var sitesPollTimer = null;

      function setToken(next) {
        token = (next || '').trim();
        if (token) {
          localStorage.setItem('nusantara_token', token);
          tokenBox.textContent = token;
          authStatus.textContent = 'Authenticated';
          authStatus.className = 'ok';
          fetchPanelVersion(true);
          fetchSites(true);
          fetchUpdateStatus(true);
          startUpdatePolling();
          startSitesPolling();
        } else {
          localStorage.removeItem('nusantara_token');
          tokenBox.textContent = '(empty)';
          authStatus.textContent = 'Not authenticated';
          authStatus.className = 'bad';
          versionMeta.textContent = 'Login as admin to read installed version.';
          stopUpdatePolling();
          stopSitesPolling();
          setUpdateState('Not authenticated', 'err', 0);
          updateMeta.textContent = 'Login as admin to use panel update.';
          sitesMeta.textContent = 'Login to load sites list.';
          sitesList.innerHTML = '<div class="site-row"><div class="muted">No data loaded.</div></div>';
        }
      }

      function setUpdateState(label, kind, progress) {
        updateState.textContent = label;
        updateState.className = 'pill ' + (kind || '');
        updateProgress.style.width = String(Math.max(0, Math.min(100, progress || 0))) + '%';
      }

      function pretty(data) {
        try { return JSON.stringify(data, null, 2); } catch (_) { return String(data); }
      }

      function startUpdatePolling() {
        if (updatePollTimer) return;
        updatePollTimer = setInterval(function () {
          fetchUpdateStatus(true);
        }, 4000);
      }

      function stopUpdatePolling() {
        if (!updatePollTimer) return;
        clearInterval(updatePollTimer);
        updatePollTimer = null;
      }

      function startSitesPolling() {
        if (sitesPollTimer) return;
        sitesPollTimer = setInterval(function () {
          fetchSites(true);
        }, 8000);
      }

      function stopSitesPolling() {
        if (!sitesPollTimer) return;
        clearInterval(sitesPollTimer);
        sitesPollTimer = null;
      }

      function escapeHtml(value) {
        return String(value || '')
          .replace(/&/g, '&amp;')
          .replace(/</g, '&lt;')
          .replace(/>/g, '&gt;')
          .replace(/"/g, '&quot;')
          .replace(/'/g, '&#039;');
      }

      function renderSites(items) {
        if (!Array.isArray(items) || items.length === 0) {
          sitesList.innerHTML = '<div class="site-row"><div class="muted">No sites yet.</div></div>';
          return;
        }
        var html = items.map(function (site) {
          var status = String(site.status || '').toLowerCase();
          var badgeClass = 'badge ' + status;
          return '' +
            '<div class="site-row">' +
              '<div class="site-main">' +
                '<div class="site-domain">' + escapeHtml(site.domain) + '</div>' +
                '<div class="site-meta">id=' + escapeHtml(site.id) + '</div>' +
                '<div class="site-meta">runtime=' + escapeHtml(site.runtime) + ' root=' + escapeHtml(site.root_path) + '</div>' +
                '<div class="' + badgeClass + '">' + escapeHtml(status || 'unknown') + '</div>' +
              '</div>' +
              '<div>' +
                '<button class="mini" data-delete-site="' + escapeHtml(site.id) + '">Delete</button>' +
              '</div>' +
            '</div>';
        }).join('');
        sitesList.innerHTML = html;
      }

      function applyUpdateStatus(st) {
        if (!st || typeof st !== 'object') return;
        var label = 'Unknown';
        var kind = '';
        var progress = 10;
        if (st.running) {
          label = 'Updating';
          kind = 'run';
          progress = 60;
        } else if (st.success) {
          label = 'Success';
          kind = 'ok';
          progress = 100;
        } else if (st.failed) {
          label = 'Failed';
          kind = 'err';
          progress = 100;
        } else if (st.exists) {
          label = (st.active_state || 'idle') + '/' + (st.sub_state || '-');
          kind = '';
          progress = 30;
        }
        setUpdateState(label, kind, progress);
        updateMeta.textContent = 'unit=' + (st.unit || '-') + ' active=' + (st.active_state || '-') + ' result=' + (st.result || '-');
        btnStartUpdate.disabled = !!st.running;
      }

      async function fetchUpdateStatus(silent) {
        if (!token) return null;
        try {
          var st = await callAPI('/v1/panel/update/status', 'GET', null, true, !!silent);
          applyUpdateStatus(st);
          return st;
        } catch (err) {
          if (!silent) {
            out.textContent = 'Request failed: ' + err;
          } else {
            setUpdateState('Reconnecting', 'run', 75);
            updateMeta.textContent = 'Panel may be restarting...';
          }
          return null;
        }
      }

      function recoverAfterUpdateTriggerDrop() {
        setUpdateState('Applying', 'run', 55);
        updateMeta.textContent = 'Connection dropped while triggering update. Checking updater status...';
        startUpdatePolling();
        fetchUpdateStatus(true);
      }

      function applyPanelVersion(info) {
        if (!info || typeof info !== 'object') return;
        versionMeta.textContent =
          'version=' + (info.version || '-') +
          ' commit=' + (info.commit || '-') +
          ' build_time=' + (info.build_time || '-');
      }

      async function fetchPanelVersion(silent) {
        if (!token) return null;
        try {
          var info = await callAPI('/v1/panel/version', 'GET', null, true, !!silent);
          applyPanelVersion(info);
          return info;
        } catch (err) {
          if (!silent) {
            out.textContent = 'Request failed: ' + err;
          }
          return null;
        }
      }

      async function fetchSites(silent) {
        if (!token) return null;
        try {
          var payload = await callAPI('/v1/sites', 'GET', null, true, !!silent);
          var items = (payload && payload.items) || [];
          renderSites(items);
          sitesMeta.textContent = 'Sites loaded: ' + items.length + ' (auto-refresh every 8s)';
          return payload;
        } catch (err) {
          if (!silent) {
            out.textContent = 'Request failed: ' + err;
          }
          return null;
        }
      }

      async function callAPI(path, method, body, needAuth, silent) {
        var headers = { 'Content-Type': 'application/json' };
        if (needAuth && token) {
          headers.Authorization = 'Bearer ' + token;
        }
        var opts = { method: method || 'GET', headers: headers };
        if (body) {
          opts.body = JSON.stringify(body);
        }
        var res = await fetch(path, opts);
        var ct = res.headers.get('content-type') || '';
        var payload = ct.indexOf('application/json') >= 0 ? await res.json() : await res.text();
        if (!silent) {
          out.textContent = 'HTTP ' + res.status + '\n' + pretty(payload);
        }
        return payload;
      }

      document.getElementById('btnLogin').addEventListener('click', async function () {
        try {
          var payload = await callAPI('/v1/auth/login', 'POST', {
            username: document.getElementById('username').value,
            password: document.getElementById('password').value
          }, false);
          if (payload && payload.token) {
            setToken(payload.token);
          }
        } catch (err) {
          out.textContent = 'Request failed: ' + err;
        }
      });

      document.getElementById('btnLogout').addEventListener('click', async function () {
        try {
          await callAPI('/v1/auth/logout', 'POST', null, true);
        } catch (err) {
          out.textContent = 'Request failed: ' + err;
        }
        setToken('');
      });

      document.getElementById('btnMe').addEventListener('click', function () {
        callAPI('/v1/auth/me', 'GET', null, true).catch(function (err) {
          out.textContent = 'Request failed: ' + err;
        });
      });
      document.getElementById('btnHealth').addEventListener('click', function () {
        callAPI('/healthz', 'GET', null, false).catch(function (err) {
          out.textContent = 'Request failed: ' + err;
        });
      });
      document.getElementById('btnCompat').addEventListener('click', function () {
        callAPI('/v1/system/compatibility', 'GET', null, false).catch(function (err) {
          out.textContent = 'Request failed: ' + err;
        });
      });
      document.getElementById('btnSites').addEventListener('click', function () {
        fetchSites(false).catch(function (err) {
          out.textContent = 'Request failed: ' + err;
        });
      });
      document.getElementById('btnJobs').addEventListener('click', function () {
        callAPI('/v1/jobs', 'GET', null, true).catch(function (err) {
          out.textContent = 'Request failed: ' + err;
        });
      });
      document.getElementById('btnPanelVersion').addEventListener('click', function () {
        fetchPanelVersion(false).catch(function (err) {
          out.textContent = 'Request failed: ' + err;
        });
      });
      document.getElementById('btnSitesRefresh').addEventListener('click', function () {
        fetchSites(false).catch(function (err) {
          out.textContent = 'Request failed: ' + err;
        });
      });
      document.getElementById('btnStartUpdate').addEventListener('click', async function () {
        try {
          var payload = await callAPI('/v1/panel/update', 'POST', {}, true);
          if (payload && payload.update_status) {
            applyUpdateStatus(payload.update_status);
          } else {
            setUpdateState('Triggered', 'run', 40);
            updateMeta.textContent = 'Updater job submitted. Polling status...';
            fetchUpdateStatus(true);
          }
          startUpdatePolling();
        } catch (err) {
          recoverAfterUpdateTriggerDrop();
          out.textContent = 'Request interrupted (panel may restart during update trigger). Auto-checking updater status...';
        }
      });
      document.getElementById('btnUpdateStatus').addEventListener('click', function () {
        fetchUpdateStatus(false).catch(function (err) {
          out.textContent = 'Request failed: ' + err;
        });
      });

      document.getElementById('btnCreateSite').addEventListener('click', function () {
        callAPI('/v1/sites', 'POST', {
          domain: document.getElementById('domain').value,
          root_path: document.getElementById('rootPath').value,
          runtime: document.getElementById('runtime').value
        }, true).then(function () {
          fetchSites(true);
        }).catch(function (err) {
          out.textContent = 'Request failed: ' + err;
        });
      });

      sitesList.addEventListener('click', function (evt) {
        var target = evt.target;
        if (!target || !target.getAttribute) return;
        var siteID = target.getAttribute('data-delete-site');
        if (!siteID) return;
        if (!window.confirm('Delete site ' + siteID + '?')) return;

        callAPI('/v1/sites/' + encodeURIComponent(siteID), 'DELETE', null, true).then(function () {
          sitesMeta.textContent = 'Delete requested for site ' + siteID + '. Refreshing...';
          fetchSites(true);
        }).catch(function (err) {
          out.textContent = 'Request failed: ' + err;
        });
      });

      setToken(token);
    })();
  </script>
</body>
</html>
`

// WebUI serves a lightweight in-process page for manual API testing.
func WebUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(uiHTML))
}
