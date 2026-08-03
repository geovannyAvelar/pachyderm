import './style.css';

import {
    ListInstalled,
    ListAvailable,
    Install,
    Use,
    Uninstall,
    InitDataDir,
    StartServer,
    StopServer,
    OpenPsql,
    GetLogs,
} from '../wailsjs/go/main/App';
import {EventsOn} from '../wailsjs/runtime/runtime';

document.querySelector('#app').innerHTML = `
  <header>
    <h1>🐘 Pachyderm</h1>
    <p>Local PostgreSQL versions, managed.</p>
  </header>

  <div class="panel">
    <h2>Installed versions</h2>
    <div class="versions" id="versions"></div>
  </div>

  <div class="panel">
    <h2>Install a version</h2>
    <div class="install-row">
      <select id="available"></select>
      <button class="primary" id="install-btn">Install</button>
    </div>
  </div>

  <div class="panel log" id="log"></div>

  <div class="modal-overlay" id="logs-overlay" hidden>
    <div class="modal">
      <div class="modal-header">
        <h2 id="logs-title">Logs</h2>
        <span class="spacer"></span>
        <button id="logs-refresh">Refresh</button>
        <button id="logs-close">Close</button>
      </div>
      <pre class="log-view" id="logs-body"></pre>
    </div>
  </div>
`;

const versionsEl = document.getElementById('versions');
const availableEl = document.getElementById('available');
const installBtn = document.getElementById('install-btn');
const logEl = document.getElementById('log');

const logsOverlay = document.getElementById('logs-overlay');
const logsTitle = document.getElementById('logs-title');
const logsBody = document.getElementById('logs-body');
const logsRefreshBtn = document.getElementById('logs-refresh');
const logsCloseBtn = document.getElementById('logs-close');

let logsVersion = null;
let logsTimer = null;

async function refreshLogs() {
    if (!logsVersion) return;
    try {
        const text = await GetLogs(logsVersion);
        logsBody.textContent = text || '(empty)';
        logsBody.scrollTop = logsBody.scrollHeight;
    } catch (err) {
        logsBody.textContent = errorMessage(err);
    }
}

function openLogs(version) {
    logsVersion = version;
    logsTitle.textContent = `Logs — PostgreSQL ${version}`;
    logsOverlay.hidden = false;
    refreshLogs();
    logsTimer = setInterval(refreshLogs, 2000);
}

function closeLogs() {
    logsOverlay.hidden = true;
    logsVersion = null;
    clearInterval(logsTimer);
    logsTimer = null;
}

logsRefreshBtn.addEventListener('click', refreshLogs);
logsCloseBtn.addEventListener('click', closeLogs);

function log(message, isError = false) {
    const line = document.createElement('div');
    line.className = 'log-line' + (isError ? ' error' : '');
    line.textContent = message;
    logEl.appendChild(line);
    logEl.scrollTop = logEl.scrollHeight;
}

function errorMessage(err) {
    if (err && typeof err === 'object' && 'message' in err) return err.message;
    return String(err);
}

async function refreshInstalled() {
    try {
        const versions = await ListInstalled();
        renderInstalled(versions || []);
    } catch (err) {
        log(errorMessage(err), true);
    }
}

async function refreshAvailable() {
    try {
        const catalog = await ListAvailable();
        availableEl.innerHTML = '';
        for (const v of catalog) {
            const opt = document.createElement('option');
            opt.value = v.version;
            opt.textContent = v.version + (v.supported ? '' : ' (unsupported)');
            availableEl.appendChild(opt);
        }
    } catch (err) {
        log(errorMessage(err), true);
    }
}

function renderInstalled(versions) {
    versionsEl.innerHTML = '';

    if (versions.length === 0) {
        versionsEl.innerHTML = '<div class="empty">No versions installed yet. Install one below.</div>';
        return;
    }

    for (const v of versions) {
        const row = document.createElement('div');
        row.className = 'version-row';

        row.innerHTML = `
      <span class="version-number">${v.version}</span>
      ${v.current ? '<span class="badge badge-current">current</span>' : ''}
      ${v.running
            ? `<span class="badge badge-running">running (PID ${v.pid})</span>`
            : '<span class="badge badge-stopped">stopped</span>'}
      <span class="spacer"></span>
    `;

        if (!v.current) {
            row.appendChild(makeButton('Use', () => runAction(() => Use(v.version), `Set ${v.version} as current`)));
        }

        if (!v.initialized) {
            row.appendChild(makeButton('Initialize', () => runAction(() => InitDataDir(v.version), `Initialize ${v.version}`)));
        } else if (v.running) {
            row.appendChild(makeButton('Stop', () => runAction(() => StopServer(v.version), `Stop ${v.version}`)));
            row.appendChild(makeButton('Open psql', () => runAction(() => OpenPsql(v.version), `Open psql for ${v.version}`)));
        } else {
            row.appendChild(makeButton('Start', () => runAction(() => StartServer(v.version), `Start ${v.version}`)));
        }

        if (v.initialized) {
            row.appendChild(makeButton('Logs', () => openLogs(v.version)));
        }

        const uninstall = makeButton('Uninstall', () => runAction(() => Uninstall(v.version), `Uninstall ${v.version}`));
        uninstall.classList.add('danger');
        uninstall.disabled = v.running;
        row.appendChild(uninstall);

        versionsEl.appendChild(row);
    }
}

function makeButton(label, onClick) {
    const btn = document.createElement('button');
    btn.textContent = label;
    btn.addEventListener('click', onClick);
    return btn;
}

async function runAction(fn, description) {
    try {
        await fn();
        log(`${description}: done.`);
    } catch (err) {
        log(`${description}: ${errorMessage(err)}`, true);
    } finally {
        await refreshInstalled();
    }
}

installBtn.addEventListener('click', () => {
    const version = availableEl.value;
    if (!version) return;
    installBtn.disabled = true;
    runAction(() => Install(version), `Install ${version}`).finally(() => {
        installBtn.disabled = false;
    });
});

EventsOn('log', (message) => log(message));

refreshAvailable();
refreshInstalled();
