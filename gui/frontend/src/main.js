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
    GetConfigFiles,
    RevealConfigFile,
    ListExtensions,
    InstallExtension,
    UninstallExtension,
    Quit,
    Version,
    GetAutostartEnabled,
    SetAutostartEnabled,
    SetConfiguredPort,
} from '../wailsjs/go/main/App';
import {EventsOn} from '../wailsjs/runtime/runtime';

document.querySelector('#app').innerHTML = `
  <header>
    <div>
      <h1>🐘 Pachyderm <span class="app-version" id="app-version"></span></h1>
      <p>Local PostgreSQL versions, managed.</p>
    </div>
    <span class="spacer"></span>
    <button id="settings-btn">Settings</button>
    <button class="danger" id="quit-btn">Quit</button>
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

  <div class="modal-overlay" id="config-overlay" hidden>
    <div class="modal settings-modal">
      <div class="modal-header">
        <h2 id="config-title">Config files</h2>
        <span class="spacer"></span>
        <button id="config-close">Close</button>
      </div>
      <div class="config-list" id="config-body"></div>
    </div>
  </div>

  <div class="modal-overlay" id="extensions-overlay" hidden>
    <div class="modal">
      <div class="modal-header">
        <h2 id="extensions-title">Extensions</h2>
        <span class="spacer"></span>
        <button id="extensions-refresh">Refresh</button>
        <button id="extensions-close">Close</button>
      </div>
      <input type="text" class="search-input" id="extensions-search" placeholder="Search extensions..." />
      <div class="extensions-list" id="extensions-body"></div>
    </div>
  </div>

  <div class="modal-overlay" id="settings-overlay" hidden>
    <div class="modal settings-modal">
      <div class="modal-header">
        <h2>Settings</h2>
        <span class="spacer"></span>
        <button id="settings-close">Close</button>
      </div>
      <label class="toggle-row">
        <input type="checkbox" id="autostart-toggle" />
        <span>Start Pachyderm when you log in</span>
      </label>
      <div class="settings-error" id="settings-error" hidden></div>
    </div>
  </div>
`;

const versionsEl = document.getElementById('versions');
const availableEl = document.getElementById('available');
const installBtn = document.getElementById('install-btn');
const logEl = document.getElementById('log');
const appVersionEl = document.getElementById('app-version');

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

const configOverlay = document.getElementById('config-overlay');
const configTitle = document.getElementById('config-title');
const configBody = document.getElementById('config-body');
const configCloseBtn = document.getElementById('config-close');

const configFileLabels = [
    ['dataDir', 'Data directory'],
    ['configFile', 'postgresql.conf'],
    ['hbaFile', 'pg_hba.conf'],
    ['identFile', 'pg_ident.conf'],
];

async function openConfig(version) {
    configTitle.textContent = `Config files — PostgreSQL ${version}`;
    configBody.innerHTML = '';
    configOverlay.hidden = false;

    try {
        const files = await GetConfigFiles(version);
        for (const [key, label] of configFileLabels) {
            const row = document.createElement('div');
            row.className = 'config-row';
            row.innerHTML = `
        <div class="config-info">
          <span class="config-label">${label}</span>
          <div class="config-path">${files[key]}</div>
        </div>
      `;
            row.appendChild(makeButton('Reveal', () => RevealConfigFile(files[key])));
            configBody.appendChild(row);
        }
    } catch (err) {
        configBody.innerHTML = `<div class="empty">${errorMessage(err)}</div>`;
    }
}

function closeConfig() {
    configOverlay.hidden = true;
}

configCloseBtn.addEventListener('click', closeConfig);

const extensionsOverlay = document.getElementById('extensions-overlay');
const extensionsTitle = document.getElementById('extensions-title');
const extensionsBody = document.getElementById('extensions-body');
const extensionsRefreshBtn = document.getElementById('extensions-refresh');
const extensionsCloseBtn = document.getElementById('extensions-close');
const extensionsSearch = document.getElementById('extensions-search');

let extensionsVersion = null;
let extensionsCache = [];

async function refreshExtensions() {
    if (!extensionsVersion) return;
    try {
        extensionsCache = await ListExtensions(extensionsVersion) || [];
        renderFilteredExtensions();
    } catch (err) {
        extensionsCache = [];
        extensionsBody.innerHTML = `<div class="empty">${errorMessage(err)}</div>`;
    }
}

function renderFilteredExtensions() {
    const query = extensionsSearch.value.trim().toLowerCase();
    const filtered = query
        ? extensionsCache.filter((e) => e.name.toLowerCase().includes(query) || e.comment.toLowerCase().includes(query))
        : extensionsCache;
    renderExtensions(filtered);
}

function renderExtensions(extensions) {
    extensionsBody.innerHTML = '';

    if (extensions.length === 0) {
        const message = extensionsCache.length === 0 ? 'No extensions available.' : 'No extensions match your search.';
        extensionsBody.innerHTML = `<div class="empty">${message}</div>`;
        return;
    }

    for (const e of extensions) {
        const row = document.createElement('div');
        row.className = 'extension-row';

        row.innerHTML = `
      <div class="extension-info">
        <span class="extension-name">${e.name}</span>
        ${e.installed ? `<span class="badge badge-current">${e.version}</span>` : ''}
        <div class="extension-comment">${e.comment}</div>
      </div>
    `;

        const action = e.installed
            ? () => runExtensionAction(UninstallExtension, e.name, `Uninstall ${e.name}`)
            : () => runExtensionAction(InstallExtension, e.name, `Install ${e.name}`);
        const btn = makeButton(e.installed ? 'Uninstall' : 'Install', action);
        btn.classList.add(e.installed ? 'danger' : 'primary');
        row.appendChild(btn);

        extensionsBody.appendChild(row);
    }
}

async function runExtensionAction(fn, name, description) {
    try {
        await fn(extensionsVersion, name);
        log(`${description}: done.`);
    } catch (err) {
        log(`${description}: ${errorMessage(err)}`, true);
    } finally {
        await refreshExtensions();
    }
}

function openExtensions(version) {
    extensionsVersion = version;
    extensionsTitle.textContent = `Extensions — PostgreSQL ${version}`;
    extensionsSearch.value = '';
    extensionsOverlay.hidden = false;
    refreshExtensions();
}

function closeExtensions() {
    extensionsOverlay.hidden = true;
    extensionsVersion = null;
    extensionsCache = [];
}

extensionsRefreshBtn.addEventListener('click', refreshExtensions);
extensionsCloseBtn.addEventListener('click', closeExtensions);
extensionsSearch.addEventListener('input', renderFilteredExtensions);

const settingsOverlay = document.getElementById('settings-overlay');
const settingsCloseBtn = document.getElementById('settings-close');
const autostartToggle = document.getElementById('autostart-toggle');
const settingsError = document.getElementById('settings-error');

async function openSettings() {
    settingsError.hidden = true;
    settingsOverlay.hidden = false;
    try {
        autostartToggle.checked = await GetAutostartEnabled();
    } catch (err) {
        settingsError.hidden = false;
        settingsError.textContent = errorMessage(err);
    }
}

function closeSettings() {
    settingsOverlay.hidden = true;
}

autostartToggle.addEventListener('change', async () => {
    const desired = autostartToggle.checked;
    autostartToggle.disabled = true;
    settingsError.hidden = true;
    try {
        await SetAutostartEnabled(desired);
    } catch (err) {
        autostartToggle.checked = !desired;
        settingsError.hidden = false;
        settingsError.textContent = errorMessage(err);
    } finally {
        autostartToggle.disabled = false;
    }
});

document.getElementById('settings-btn').addEventListener('click', openSettings);
settingsCloseBtn.addEventListener('click', closeSettings);

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
            ? `<span class="badge badge-running">running (PID ${v.pid}, port ${v.port})</span>`
            : '<span class="badge badge-stopped">stopped</span>'}
      <span class="spacer"></span>
    `;

        if (!v.current) {
            row.appendChild(makeButton('Use', () => runAction(() => Use(v.version), `Set ${v.version} as current`)));
        }

        if (!v.running) {
            row.appendChild(makePortEditor(v.version, v.configuredPort));
        }

        if (!v.initialized) {
            row.appendChild(makeButton('Initialize', () => runAction(() => InitDataDir(v.version), `Initialize ${v.version}`)));
        } else if (v.running) {
            row.appendChild(makeButton('Stop', () => runAction(() => StopServer(v.version), `Stop ${v.version}`)));
            row.appendChild(makeButton('Open psql', () => runAction(() => OpenPsql(v.version), `Open psql for ${v.version}`)));
            row.appendChild(makeButton('Extensions', () => openExtensions(v.version)));
        } else {
            row.appendChild(makeButton('Start', () => runAction(() => StartServer(v.version), `Start ${v.version}`)));
        }

        if (v.initialized) {
            row.appendChild(makeButton('Logs', () => openLogs(v.version)));
            row.appendChild(makeButton('Config', () => openConfig(v.version)));
        }

        const uninstall = makeButton('Uninstall', () => runAction(() => Uninstall(v.version), `Uninstall ${v.version}`));
        uninstall.classList.add('danger');
        uninstall.disabled = v.running;
        row.appendChild(uninstall);

        versionsEl.appendChild(row);
    }
}

function makePortEditor(version, configuredPort) {
    const wrap = document.createElement('div');
    wrap.className = 'port-editor';

    const input = document.createElement('input');
    input.type = 'number';
    input.className = 'port-input';
    input.min = 1024;
    input.max = 65535;
    input.value = configuredPort;
    input.title = 'Port this version starts on';
    wrap.appendChild(input);

    const saveBtn = makeButton('Save port', async () => {
        const port = parseInt(input.value, 10);
        saveBtn.disabled = true;
        try {
            await SetConfiguredPort(version, port);
            log(`Port for ${version} set to ${port}.`);
        } catch (err) {
            log(`Set port for ${version}: ${errorMessage(err)}`, true);
        } finally {
            saveBtn.disabled = false;
        }
    });
    wrap.appendChild(saveBtn);

    return wrap;
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

document.getElementById('quit-btn').addEventListener('click', () => Quit());

EventsOn('log', (message) => log(message));

Version().then((v) => {
    appVersionEl.textContent = `v${v}`;
});

refreshAvailable();
refreshInstalled();
