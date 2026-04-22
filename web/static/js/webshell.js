// English note.

const WEBSHELL_SIDEBAR_WIDTH_KEY = 'webshell_sidebar_width';
const WEBSHELL_DEFAULT_SIDEBAR_WIDTH = 360;
/* English note. */
const WEBSHELL_MAIN_MIN_WIDTH = 380;
const WEBSHELL_PROMPT = 'shell> ';
let webshellConnections = [];
let currentWebshellId = null;
let webshellTerminalInstance = null;
let webshellTerminalFitAddon = null;
let webshellTerminalResizeObserver = null;
let webshellTerminalResizeContainer = null;
let webshellCurrentConn = null;
let webshellLineBuffer = '';
let webshellRunning = false;
let webshellTerminalRunning = false;
let webshellTerminalLogsByConn = {};
let webshellTerminalSessionsByConn = {};
let webshellPersistLoadedByConn = {};
let webshellPersistSaveTimersByConn = {};
// English note.
let webshellHistoryByConn = {};
let webshellHistoryIndex = -1;
const WEBSHELL_HISTORY_MAX = 100;
// English note.
let webshellClearInProgress = false;
// English note.
let webshellAiConvMap = {};
let webshellAiSending = false;
let webshellAiAbortController = null; // AbortController for current AI stream
let webshellAiStreamReader = null;    // Current ReadableStreamDefaultReader
let webshellDbConfigByConn = {};
let webshellDirTreeByConn = {};
let webshellDirExpandedByConn = {};
let webshellDirLoadedByConn = {};
// English note.
let webshellStreamingTypingId = 0;
let webshellProbeStatusById = {};
let webshellBatchProbeRunning = false;

/* English note. */
function resolveWebshellAiStreamRequest() {
    if (typeof apiFetch === 'undefined') {
        return Promise.resolve({ path: '/api/agent-loop/stream', orchestration: null });
    }
    return apiFetch('/api/config').then(function (r) {
        if (!r.ok) return null;
        return r.json();
    }).then(function (cfg) {
        var norm = null;
        if (typeof window.csaiChatAgentMode === 'object' && typeof window.csaiChatAgentMode.normalizeStored === 'function') {
            norm = window.csaiChatAgentMode.normalizeStored(localStorage.getItem('cyberstrike-chat-agent-mode'), cfg);
        } else {
            var mode = localStorage.getItem('cyberstrike-chat-agent-mode');
            if (mode === 'single') mode = 'react';
            if (mode === 'multi') mode = 'deep';
            norm = mode || 'react';
        }
        if (typeof window.csaiChatAgentMode === 'object' && typeof window.csaiChatAgentMode.isEinoSingle === 'function' && window.csaiChatAgentMode.isEinoSingle(norm)) {
            return { path: '/api/eino-agent/stream', orchestration: null };
        }
        if (!cfg || !cfg.multi_agent || !cfg.multi_agent.enabled) {
            return { path: '/api/agent-loop/stream', orchestration: null };
        }
        if (typeof window.csaiChatAgentMode === 'object' && typeof window.csaiChatAgentMode.isEino === 'function' && window.csaiChatAgentMode.isEino(norm)) {
            return { path: '/api/multi-agent/stream', orchestration: norm };
        }
        return { path: '/api/agent-loop/stream', orchestration: null };
    }).catch(function () {
        return { path: '/api/agent-loop/stream', orchestration: null };
    });
}

// English note.

let wsRolesCache = null; //  /api/roles 

function wsLoadRoles() {
    if (typeof apiFetch === 'undefined') return;
    apiFetch('/api/roles').then(function (r) { return r.json(); }).then(function (data) {
        wsRolesCache = (data && Array.isArray(data.roles)) ? data.roles : [];
        wsRenderRoleList();
        wsUpdateRoleSelectorDisplay();
    }).catch(function () { /* ignore */ });
}

function wsUpdateRoleSelectorDisplay() {
    var iconEl = document.getElementById('ws-role-selector-icon');
    var textEl = document.getElementById('ws-role-selector-text');
    if (!iconEl || !textEl) return;
    var cur = (typeof getCurrentRole === 'function') ? getCurrentRole() : (localStorage.getItem('currentRole') || '');
    if (!cur) {
        iconEl.textContent = '\ud83d\udd35';
        textEl.textContent = (typeof window.t === 'function' ? window.t('chat.defaultRole') : '') || '';
        return;
    }
    if (wsRolesCache) {
        for (var i = 0; i < wsRolesCache.length; i++) {
            if (wsRolesCache[i].name === cur) {
                iconEl.textContent = wsRolesCache[i].icon || '\ud83d\udd35';
                textEl.textContent = cur;
                return;
            }
        }
    }
    iconEl.textContent = '\ud83d\udd35';
    textEl.textContent = cur;
}

function wsRenderRoleList() {
    var listEl = document.getElementById('ws-role-selection-list');
    if (!listEl) return;
    var cur = (typeof getCurrentRole === 'function') ? getCurrentRole() : (localStorage.getItem('currentRole') || '');
    var html = '';
    // English note.
    var defSelected = !cur ? ' selected' : '';
    html += '<button type="button" class="role-selection-item-main' + defSelected + '" onclick="wsSelectRole(\'\')">' +
        '<div class="role-selection-item-icon-main">\ud83d\udd35</div>' +
        '<div class="role-selection-item-content-main"><div class="role-selection-item-name-main">' +
        (wsTOr('chat.defaultRole', '')) +
        '</div><div class="role-selection-item-description-main">' +
        (wsTOr('roles.defaultRoleDescription', '，，')) +
        '</div></div>' +
        (defSelected ? '<div class="role-selection-checkmark-main">\u2713</div>' : '') +
        '</button>';
    if (wsRolesCache) {
        for (var i = 0; i < wsRolesCache.length; i++) {
            var r = wsRolesCache[i];
            if (!r.enabled) continue;
            if (r.name === '') continue; // ， API 
            var sel = (r.name === cur) ? ' selected' : '';
            html += '<button type="button" class="role-selection-item-main' + sel + '" onclick="wsSelectRole(\'' + r.name.replace(/'/g, "\\'") + '\')">' +
                '<div class="role-selection-item-icon-main">' + (r.icon || '\ud83d\udd35') + '</div>' +
                '<div class="role-selection-item-content-main"><div class="role-selection-item-name-main">' + r.name + '</div>' +
                '<div class="role-selection-item-description-main">' + (r.description || '').substring(0, 60) + '</div></div>' +
                (sel ? '<div class="role-selection-checkmark-main">\u2713</div>' : '') +
                '</button>';
        }
    }
    listEl.innerHTML = html;
}

function wsSelectRole(name) {
    var roleName = name || '';
    // English note.
    if (typeof handleRoleChange === 'function') {
        try { handleRoleChange(roleName); } catch (e) { /* */ }
    } else {
        try { localStorage.setItem('currentRole', roleName); } catch (e) { /* */ }
    }
    if (typeof window.currentSelectedRole !== 'undefined') window.currentSelectedRole = roleName;
    wsUpdateRoleSelectorDisplay();
    wsRenderRoleList();
    wsCloseRolePanel();
}

function wsToggleRolePanel() {
    var panel = document.getElementById('ws-role-selection-panel');
    if (!panel) return;
    var isOpen = panel.style.display === 'flex';
    if (isOpen) { wsCloseRolePanel(); return; }
    wsCloseAgentModePanel();
    panel.style.display = 'flex';
}
function wsCloseRolePanel() {
    var panel = document.getElementById('ws-role-selection-panel');
    if (panel) panel.style.display = 'none';
}

// English note.

function wsInitAgentMode() {
    if (typeof apiFetch === 'undefined') return;
    apiFetch('/api/config').then(function (r) { return r.ok ? r.json() : null; }).then(function (cfg) {
        var wrapper = document.getElementById('ws-agent-mode-wrapper');
        if (!wrapper) return;
        wrapper.style.display = '';
        // English note.
        var multiOn = cfg && cfg.multi_agent && cfg.multi_agent.enabled;
        // English note.
        var opts = wrapper.querySelectorAll('.ws-agent-mode-option');
        opts.forEach(function (el) {
            var v = el.getAttribute('data-value');
            if (v === 'deep' || v === 'plan_execute' || v === 'supervisor') {
                el.style.display = multiOn ? '' : 'none';
            }
        });
        // English note.
        var stored = localStorage.getItem('cyberstrike-chat-agent-mode');
        var norm;
        if (typeof window.csaiChatAgentMode === 'object' && typeof window.csaiChatAgentMode.normalizeStored === 'function') {
            norm = window.csaiChatAgentMode.normalizeStored(stored, cfg);
        } else {
            norm = stored || 'react';
            if (norm === 'single') norm = 'react';
            if (norm === 'multi') norm = 'deep';
        }
        wsSyncAgentMode(norm);
    }).catch(function () {
        var wrapper = document.getElementById('ws-agent-mode-wrapper');
        if (wrapper) wrapper.style.display = '';
        wsSyncAgentMode('react');
    });
}

function wsSyncAgentMode(value) {
    var hid = document.getElementById('ws-agent-mode-select');
    var label = document.getElementById('ws-agent-mode-text');
    var icon = document.getElementById('ws-agent-mode-icon');
    if (hid) hid.value = value;
    if (label) label.textContent = (typeof getAgentModeLabelForValue === 'function') ? getAgentModeLabelForValue(value) : value;
    if (icon) icon.textContent = (typeof getAgentModeIconForValue === 'function') ? getAgentModeIconForValue(value) : '\ud83e\udd16';
    var wrapper = document.getElementById('ws-agent-mode-wrapper');
    if (wrapper) {
        wrapper.querySelectorAll('.ws-agent-mode-option').forEach(function (el) {
            el.classList.toggle('selected', el.getAttribute('data-value') === value);
        });
    }
}

function wsSelectAgentMode(mode) {
    try { localStorage.setItem('cyberstrike-chat-agent-mode', mode); } catch (e) { /* */ }
    wsSyncAgentMode(mode);
    wsCloseAgentModePanel();
    // English note.
    if (typeof syncAgentModeFromValue === 'function') try { syncAgentModeFromValue(mode); } catch (e) { /* */ }
}

function wsToggleAgentModePanel() {
    var panel = document.getElementById('ws-agent-mode-panel');
    if (!panel) return;
    var isOpen = panel.style.display === 'flex';
    if (isOpen) { wsCloseAgentModePanel(); return; }
    wsCloseRolePanel();
    panel.style.display = 'flex';
}
function wsCloseAgentModePanel() {
    var panel = document.getElementById('ws-agent-mode-panel');
    if (panel) panel.style.display = 'none';
}

/* English note. */
function wsRefreshSelectors() {
    wsUpdateRoleSelectorDisplay();
    wsRenderRoleList();
    var stored = localStorage.getItem('cyberstrike-chat-agent-mode') || 'react';
    wsSyncAgentMode(stored);
}

// English note.
document.addEventListener('click', function (e) {
    var rolePanel = document.getElementById('ws-role-selection-panel');
    var roleBtn = document.getElementById('ws-role-selector-btn');
    if (rolePanel && rolePanel.style.display !== 'none' && roleBtn && !rolePanel.contains(e.target) && !roleBtn.contains(e.target)) {
        wsCloseRolePanel();
    }
    var modePanel = document.getElementById('ws-agent-mode-panel');
    var modeBtn = document.getElementById('ws-agent-mode-btn');
    if (modePanel && modePanel.style.display !== 'none' && modeBtn && !modePanel.contains(e.target) && !modeBtn.contains(e.target)) {
        wsCloseAgentModePanel();
    }
});

// English note.

/* English note. */
function wsStopAiStream(conn) {
    // 1. Abort the fetch
    if (webshellAiAbortController) {
        try { webshellAiAbortController.abort(); } catch (e) { /* */ }
        webshellAiAbortController = null;
    }
    // 2. Cancel the reader
    if (webshellAiStreamReader) {
        try { webshellAiStreamReader.cancel(); } catch (e) { /* */ }
        webshellAiStreamReader = null;
    }
    // 3. Call backend cancel API if we have a conversation
    var convId = conn && conn.id ? (webshellAiConvMap[conn.id] || '') : '';
    if (convId && typeof apiFetch === 'function') {
        apiFetch('/api/agent-loop/cancel', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ conversationId: convId })
        }).catch(function () { /* ignore */ });
    }
    // 4. Reset UI state
    wsSetAiSendingState(false);
}

/* English note. */
function wsSetAiSendingState(sending) {
    webshellAiSending = sending;
    var sendBtn = document.getElementById('webshell-ai-send');
    var stopBtn = document.getElementById('webshell-ai-stop');
    if (sendBtn) {
        sendBtn.disabled = sending;
        sendBtn.style.display = sending ? 'none' : '';
    }
    if (stopBtn) {
        stopBtn.style.display = sending ? '' : 'none';
    }
}

// English note.
function getWebshellConnections() {
    if (typeof apiFetch === 'undefined') {
        return Promise.resolve([]);
    }
    return apiFetch('/api/webshell/connections', { method: 'GET' })
        .then(function (r) { return r.json(); })
        .then(function (list) { return Array.isArray(list) ? list : []; })
        .catch(function (e) {
            console.warn(' WebShell ', e);
            return [];
        });
}

// English note.
function refreshWebshellConnectionsFromServer() {
    return getWebshellConnections().then(function (list) {
        webshellConnections = list;
        renderWebshellList();
        return list;
    });
}

// English note.
function wsT(key) {
    var globalT = typeof window !== 'undefined' ? window.t : null;
    if (typeof globalT === 'function' && globalT !== wsT) return globalT(key);
    var fallback = {
        'webshell.title': 'WebShell ',
        'webshell.addConnection': '',
        'webshell.cmdParam': '',
        'webshell.cmdParamPlaceholder': ' cmd， xxx  xxx=',
        'webshell.connections': '',
        'webshell.noConnections': '，「」',
        'webshell.selectOrAdd': '， WebShell ',
        'webshell.deleteConfirm': '？',
        'webshell.editConnection': '',
        'webshell.editConnectionTitle': '',
        'webshell.tabTerminal': '',
        'webshell.tabFileManager': '',
        'webshell.tabAiAssistant': 'AI ',
        'webshell.tabDbManager': '',
        'webshell.tabMemo': '',
        'webshell.dbType': '',
        'webshell.dbHost': '',
        'webshell.dbPort': '',
        'webshell.dbUsername': '',
        'webshell.dbPassword': '',
        'webshell.dbName': '',
        'webshell.dbSqlitePath': 'SQLite ',
        'webshell.dbSqlPlaceholder': ' SQL，：SELECT version();',
        'webshell.dbRunSql': ' SQL',
        'webshell.dbTest': '',
        'webshell.dbOutput': '',
        'webshell.dbNoConn': ' WebShell ',
        'webshell.dbSqlRequired': ' SQL',
        'webshell.dbRunning': '，',
        'webshell.dbCliHint': '，（mysql/psql/sqlite3/sqlcmd）',
        'webshell.dbExecFailed': '',
        'webshell.dbSchema': '',
        'webshell.dbLoadSchema': '',
        'webshell.dbNoSchema': '，',
        'webshell.dbSelectTableHint': ' SQL',
        'webshell.dbNoColumns': '',
        'webshell.dbResultTable': '',
        'webshell.dbClearSql': ' SQL',
        'webshell.dbTemplateSql': ' SQL',
        'webshell.dbRows': '',
        'webshell.dbColumns': '',
        'webshell.dbSchemaFailed': '',
        'webshell.dbSchemaLoaded': '',
        'webshell.dbAddProfile': '',
        'webshell.dbExecSuccess': 'SQL ',
        'webshell.dbNoOutput': '（）',
        'webshell.dbRenameProfile': '',
        'webshell.dbDeleteProfile': '',
        'webshell.dbDeleteProfileConfirm': '？',
        'webshell.dbProfileNamePrompt': '',
        'webshell.dbProfileName': '',
        'webshell.dbProfiles': '',
        'webshell.aiSystemReadyMessage': '。，。',
        'webshell.aiPlaceholder': '：',
        'webshell.aiSend': '',
        'webshell.aiMemo': '',
        'webshell.aiMemoPlaceholder': '、、...',
        'webshell.aiMemoClear': '',
        'webshell.aiMemoSaving': '...',
        'webshell.aiMemoSaved': '',
        'webshell.terminalWelcome': 'WebShell  — （Ctrl+L ）',
        'webshell.quickCommands': '',
        'webshell.downloadFile': '',
        'webshell.filePath': '',
        'webshell.listDir': '',
        'webshell.readFile': '',
        'webshell.editFile': '',
        'webshell.deleteFile': '',
        'webshell.saveFile': '',
        'webshell.cancelEdit': '',
        'webshell.parentDir': '',
        'webshell.execError': '',
        'webshell.testConnectivity': '',
        'webshell.testSuccess': '，Shell ',
        'webshell.testFailed': '',
        'webshell.testNoExpectedOutput': 'Shell ，',
        'webshell.clearScreen': '',
        'webshell.copyTerminalLog': '',
        'webshell.terminalIdle': '',
        'webshell.terminalRunning': '',
        'webshell.terminalCopyOk': '',
        'webshell.terminalCopyFail': '',
        'webshell.terminalNewWindow': '',
        'webshell.terminalWindowPrefix': '',
        'webshell.running': '…',
        'webshell.waitFinish': '',
        'webshell.newDir': '',
        'webshell.rename': '',
        'webshell.upload': '',
        'webshell.newFile': '',
        'webshell.filterPlaceholder': '',
        'webshell.batchDelete': '',
        'webshell.batchDownload': '',
        'webshell.moreActions': '',
        'webshell.refresh': '',
        'webshell.selectAll': '',
        'webshell.breadcrumbHome': '',
        'webshell.dirTree': '',
        'webshell.searchPlaceholder': '...',
        'webshell.noMatchConnections': '',
        'webshell.batchProbe': '',
        'webshell.probeRunning': '',
        'webshell.probeOnline': '',
        'webshell.probeOffline': '',
        'webshell.probeNoConnections': '',
        'webshell.back': '',
        'webshell.colModifiedAt': '',
        'webshell.colPerms': '',
        'webshell.colOwner': '',
        'webshell.colGroup': '',
        'webshell.colType': '',
        'common.delete': '',
        'common.refresh': '',
        'common.actions': ''
    };
    return fallback[key] || key;
}

function wsTOr(key, fallbackText) {
    var text = wsT(key);
    if (!text || text === key) return fallbackText;
    return text;
}

// English note.
function bindWebshellClearOnce() {
    if (window._webshellClearBound) return;
    window._webshellClearBound = true;
    document.body.addEventListener('click', function (e) {
        var btn = e.target && (e.target.id === 'webshell-terminal-clear' ? e.target : e.target.closest ? e.target.closest('#webshell-terminal-clear') : null);
        if (!btn || !webshellCurrentConn) return;
        e.preventDefault();
        e.stopPropagation();
        if (webshellClearInProgress) return;
        webshellClearInProgress = true;
        try {
            destroyWebshellTerminal();
            webshellLineBuffer = '';
            webshellHistoryIndex = -1;
            if (webshellCurrentConn && webshellCurrentConn.id) {
                var sid = getActiveWebshellTerminalSessionId(webshellCurrentConn.id);
                clearWebshellTerminalLog(getWebshellTerminalSessionKey(webshellCurrentConn.id, sid));
            }
            initWebshellTerminal(webshellCurrentConn);
        } finally {
            setTimeout(function () { webshellClearInProgress = false; }, 100);
        }
    }, true);
}

// English note.
function bindWebshellActionMenusAutoCloseOnce() {
    if (window._webshellActionMenusAutoCloseBound) return;
    window._webshellActionMenusAutoCloseBound = true;
    document.addEventListener('click', function (e) {
        // English note.
        var clickedInMenu = e.target && e.target.closest && (
            e.target.closest('details.webshell-conn-actions') ||
            e.target.closest('details.webshell-row-actions') ||
            e.target.closest('details.webshell-toolbar-actions')
        );
        if (clickedInMenu) return;

        var openDetails = document.querySelectorAll(
            'details.webshell-conn-actions[open],details.webshell-row-actions[open],details.webshell-toolbar-actions[open]'
        );
        openDetails.forEach(function (d) { d.open = false; });
    }, true);
}

// English note.
function initWebshellPage() {
    bindWebshellClearOnce();
    bindWebshellActionMenusAutoCloseOnce();
    destroyWebshellTerminal();
    webshellCurrentConn = null;
    currentWebshellId = null;
    webshellConnections = [];
    renderWebshellList();
    applyWebshellSidebarWidth();
    initWebshellSidebarResize();

    // English note.
    var searchEl = document.getElementById('webshell-conn-search');
    if (searchEl && searchEl.dataset.bound !== '1') {
        searchEl.dataset.bound = '1';
        searchEl.addEventListener('input', function () {
            renderWebshellList();
        });
    }

    const workspace = document.getElementById('webshell-workspace');
    if (workspace) {
        workspace.innerHTML = '<div class="webshell-workspace-placeholder" data-i18n="webshell.selectOrAdd">' + (wsT('webshell.selectOrAdd')) + '</div>';
    }
    getWebshellConnections().then(function (list) {
        webshellConnections = list;
        renderWebshellList();
    });

    var batchProbeBtn = document.getElementById('webshell-batch-probe-btn');
    if (batchProbeBtn && batchProbeBtn.dataset.bound !== '1') {
        batchProbeBtn.dataset.bound = '1';
        batchProbeBtn.addEventListener('click', function () {
            runBatchProbeWebshellConnections();
        });
    }
    updateWebshellBatchProbeButton();
}

function getWebshellSidebarWidth() {
    try {
        const w = parseInt(localStorage.getItem(WEBSHELL_SIDEBAR_WIDTH_KEY), 10);
        if (!isNaN(w) && w >= 260 && w <= 800) return w;
    } catch (e) {}
    return WEBSHELL_DEFAULT_SIDEBAR_WIDTH;
}

function setWebshellSidebarWidth(px) {
    localStorage.setItem(WEBSHELL_SIDEBAR_WIDTH_KEY, String(px));
}

function applyWebshellSidebarWidth() {
    const sidebar = document.getElementById('webshell-sidebar');
    if (!sidebar) return;
    const parentW = sidebar.parentElement ? sidebar.parentElement.offsetWidth : 0;
    let w = getWebshellSidebarWidth();
    if (parentW > 0) w = Math.min(w, Math.max(260, parentW - WEBSHELL_MAIN_MIN_WIDTH));
    sidebar.style.width = w + 'px';
}

function initWebshellSidebarResize() {
    const handle = document.getElementById('webshell-resize-handle');
    const sidebar = document.getElementById('webshell-sidebar');
    if (!handle || !sidebar || handle.dataset.resizeBound === '1') return;
    handle.dataset.resizeBound = '1';
    let startX = 0, startW = 0;
    function onMove(e) {
        const dx = e.clientX - startX;
        let w = Math.round(startW + dx);
        const parentW = sidebar.parentElement ? sidebar.parentElement.offsetWidth : 800;
        const min = 260;
        const max = Math.min(800, parentW - WEBSHELL_MAIN_MIN_WIDTH);
        w = Math.max(min, Math.min(max, w));
        sidebar.style.width = w + 'px';
    }
    function onUp() {
        handle.classList.remove('active');
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
        document.removeEventListener('mousemove', onMove);
        document.removeEventListener('mouseup', onUp);
        setWebshellSidebarWidth(parseInt(sidebar.style.width, 10) || WEBSHELL_DEFAULT_SIDEBAR_WIDTH);
    }
    handle.addEventListener('mousedown', function (e) {
        if (e.button !== 0) return;
        e.preventDefault();
        startX = e.clientX;
        startW = sidebar.offsetWidth;
        handle.classList.add('active');
        document.body.style.cursor = 'col-resize';
        document.body.style.userSelect = 'none';
        document.addEventListener('mousemove', onMove);
        document.addEventListener('mouseup', onUp);
    });
}

// English note.
function destroyWebshellTerminal() {
    if (webshellTerminalResizeObserver && webshellTerminalResizeContainer) {
        try { webshellTerminalResizeObserver.unobserve(webshellTerminalResizeContainer); } catch (e) {}
        webshellTerminalResizeObserver = null;
        webshellTerminalResizeContainer = null;
    }
    if (webshellTerminalInstance) {
        try {
            webshellTerminalInstance.dispose();
        } catch (e) {}
        webshellTerminalInstance = null;
    }
    webshellTerminalFitAddon = null;
    webshellLineBuffer = '';
    webshellRunning = false;
    webshellTerminalRunning = false;
    setWebshellTerminalStatus(false);
}

// English note.
function renderWebshellList() {
    const listEl = document.getElementById('webshell-list');
    if (!listEl) return;

    const searchEl = document.getElementById('webshell-conn-search');
    const searchTerm = (searchEl && typeof searchEl.value === 'string' ? searchEl.value : '').trim().toLowerCase();

    if (!webshellConnections.length) {
        listEl.innerHTML = '<div class="webshell-empty" data-i18n="webshell.noConnections">' + (wsT('webshell.noConnections')) + '</div>';
        return;
    }

    const filtered = searchTerm
        ? webshellConnections.filter(conn => {
            const id = String(conn.id || '').toLowerCase();
            const url = String(conn.url || '').toLowerCase();
            const remark = String(conn.remark || '').toLowerCase();
            return id.includes(searchTerm) || url.includes(searchTerm) || remark.includes(searchTerm);
        })
        : webshellConnections;

    if (filtered.length === 0) {
        listEl.innerHTML = '<div class="webshell-empty">' + (wsT('webshell.noMatchConnections') || '') + '</div>';
        return;
    }

    listEl.innerHTML = filtered.map(conn => {
        const remark = (conn.remark || conn.url || '').replace(/</g, '&lt;').replace(/>/g, '&gt;');
        const url = (conn.url || '').replace(/</g, '&lt;').replace(/>/g, '&gt;');
        const urlTitle = (conn.url || '').replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;');
        const active = currentWebshellId === conn.id ? ' active' : '';
        const safeId = escapeHtml(conn.id);
        const actionsLabel = wsT('common.actions') || '';
        const probe = webshellProbeStatusById[conn.id] || null;
        var probeHtml = '';
        if (probe && probe.state === 'probing') {
            probeHtml = '<span class="webshell-probe-badge probing">' + (wsT('webshell.probeRunning') || '') + '</span>';
        } else if (probe && probe.state === 'ok') {
            probeHtml = '<span class="webshell-probe-badge ok">' + (wsT('webshell.probeOnline') || '') + '</span>';
        } else if (probe && probe.state === 'fail') {
            probeHtml = '<span class="webshell-probe-badge fail" title="' + escapeHtml(probe.message || '') + '">' + (wsT('webshell.probeOffline') || '') + '</span>';
        }
        return (
            '<div class="webshell-item' + active + '" data-id="' + safeId + '">' +
            '<div class="webshell-item-remark-row"><div class="webshell-item-remark" title="' + urlTitle + '">' + remark + '</div>' + probeHtml + '</div>' +
            '<div class="webshell-item-url" title="' + urlTitle + '">' + url + '</div>' +
            '<div class="webshell-item-actions">' +
            '<details class="webshell-conn-actions"><summary class="btn-ghost btn-sm webshell-conn-actions-btn" title="' + actionsLabel + '">' + actionsLabel + '</summary>' +
            '<div class="webshell-row-actions-menu">' +
            '<button type="button" class="btn-ghost btn-sm webshell-edit-conn-btn" data-id="' + safeId + '" title="' + wsT('webshell.editConnection') + '">' + wsT('webshell.editConnection') + '</button>' +
            '<button type="button" class="btn-ghost btn-sm webshell-delete-btn" data-id="' + safeId + '" title="' + wsT('common.delete') + '">' + wsT('common.delete') + '</button>' +
            '</div></details>' +
            '</div>' +
            '</div>'
        );
    }).join('');

    listEl.querySelectorAll('.webshell-item').forEach(el => {
        el.addEventListener('click', function (e) {
            if (e.target.closest('.webshell-delete-btn') || e.target.closest('.webshell-edit-conn-btn') || e.target.closest('.webshell-conn-actions-btn')) return;
            selectWebshell(el.getAttribute('data-id'));
        });
    });
    listEl.querySelectorAll('.webshell-edit-conn-btn').forEach(btn => {
        btn.addEventListener('click', function (e) {
            e.stopPropagation();
            showEditWebshellModal(btn.getAttribute('data-id'));
        });
    });
    listEl.querySelectorAll('.webshell-delete-btn').forEach(btn => {
        btn.addEventListener('click', function (e) {
            e.stopPropagation();
            deleteWebshell(btn.getAttribute('data-id'));
        });
    });
}

function probeWebshellConnection(conn) {
    if (!conn || typeof apiFetch === 'undefined') {
        return Promise.resolve({ ok: false, message: wsT('webshell.testFailed') || '' });
    }
    return apiFetch('/api/webshell/exec', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            url: conn.url,
            password: conn.password || '',
            type: conn.type || 'php',
            method: ((conn.method || 'post').toLowerCase() === 'get') ? 'get' : 'post',
            cmd_param: conn.cmdParam || '',
            command: 'echo 1'
        })
    })
        .then(function (r) { return r.json(); })
        .then(function (data) {
            var output = (data && data.output != null) ? String(data.output).trim() : '';
            var ok = !!(data && data.ok && output === '1');
            if (ok) return { ok: true, message: wsT('webshell.testSuccess') || '，Shell ' };
            var msg = (data && data.error) ? data.error : (wsT('webshell.testFailed') || '');
            return { ok: false, message: msg };
        })
        .catch(function (e) {
            return { ok: false, message: (e && e.message) ? e.message : String(e) };
        });
}

function updateWebshellBatchProbeButton(done, total, okCount) {
    var btn = document.getElementById('webshell-batch-probe-btn');
    if (!btn) return;
    if (webshellBatchProbeRunning) {
        var d = typeof done === 'number' ? done : 0;
        var t = typeof total === 'number' ? total : webshellConnections.length;
        btn.disabled = true;
        btn.textContent = (wsT('webshell.probeRunning') || '') + ' ' + d + '/' + t;
        return;
    }
    btn.disabled = false;
    if (typeof done === 'number' && typeof total === 'number' && total > 0 && typeof okCount === 'number') {
        btn.textContent = (wsT('webshell.batchProbe') || '') + ' (' + okCount + '/' + total + ')';
    } else {
        btn.textContent = wsT('webshell.batchProbe') || '';
    }
}

function runBatchProbeWebshellConnections() {
    if (webshellBatchProbeRunning) return;
    if (!Array.isArray(webshellConnections) || webshellConnections.length === 0) {
        alert(wsT('webshell.probeNoConnections') || '');
        return;
    }
    webshellBatchProbeRunning = true;
    var total = webshellConnections.length;
    var done = 0;
    var okCount = 0;

    webshellConnections.forEach(function (conn) {
        if (!conn || !conn.id) return;
        webshellProbeStatusById[conn.id] = { state: 'probing', message: '' };
    });
    renderWebshellList();
    updateWebshellBatchProbeButton(done, total, okCount);

    var idx = 0;
    var concurrency = Math.min(4, total);

    function runOne() {
        if (idx >= total) return Promise.resolve();
        var conn = webshellConnections[idx++];
        if (!conn || !conn.id) {
            done++;
            updateWebshellBatchProbeButton(done, total, okCount);
            return runOne();
        }
        return probeWebshellConnection(conn).then(function (res) {
            if (res.ok) okCount++;
            webshellProbeStatusById[conn.id] = {
                state: res.ok ? 'ok' : 'fail',
                message: res.message || ''
            };
            done++;
            renderWebshellList();
            updateWebshellBatchProbeButton(done, total, okCount);
        }).then(runOne);
    }

    var workers = [];
    for (var i = 0; i < concurrency; i++) workers.push(runOne());
    Promise.all(workers).finally(function () {
        webshellBatchProbeRunning = false;
        updateWebshellBatchProbeButton(done, total, okCount);
    });
}

function escapeHtml(s) {
    if (!s) return '';
    const div = document.createElement('div');
    div.textContent = s;
    return div.innerHTML;
}

function escapeSingleQuotedShellArg(value) {
    var s = value == null ? '' : String(value);
    return "'" + s.replace(/'/g, "'\\''") + "'";
}

function safeConnIdForStorage(conn) {
    if (!conn || !conn.id) return '';
    return String(conn.id).replace(/[^\w.-]/g, '_');
}

function normalizeWebshellPath(path) {
    var p = path == null ? '.' : String(path).trim();
    if (!p || p === '/') return '.';
    p = p.replace(/\\/g, '/').replace(/^\/+/, '').replace(/\/+/g, '/');
    if (!p || p === '.') return '.';
    if (p.endsWith('/')) p = p.slice(0, -1);
    return p || '.';
}

function getWebshellTerminalSessionKey(connId, sessionId) {
    if (!connId || !sessionId) return '';
    return String(connId) + '::' + String(sessionId);
}

function normalizeWebshellTerminalSessions(raw) {
    var state = raw && typeof raw === 'object' ? raw : {};
    var list = Array.isArray(state.sessions) ? state.sessions.slice() : [];
    if (!list.length) {
        list = [{ id: 't1', name: (wsT('webshell.terminalWindowPrefix') || '') + '1' }];
    }
    list = list.map(function (s, i) {
        var id = (s && s.id ? String(s.id) : ('t' + (i + 1)));
        var name = (s && s.name ? String(s.name) : ((wsT('webshell.terminalWindowPrefix') || '') + (i + 1)));
        return { id: id, name: name };
    });
    var activeId = state.activeId;
    if (!activeId || !list.some(function (s) { return s.id === activeId; })) activeId = list[0].id;
    return { sessions: list, activeId: activeId };
}

function getWebshellTerminalSessions(connId) {
    if (!connId) return normalizeWebshellTerminalSessions(null);
    if (webshellTerminalSessionsByConn[connId]) return webshellTerminalSessionsByConn[connId];
    var state = normalizeWebshellTerminalSessions(null);
    webshellTerminalSessionsByConn[connId] = state;
    return state;
}

function saveWebshellTerminalSessions(connId, state) {
    if (!connId || !state) return;
    var normalized = normalizeWebshellTerminalSessions(state);
    webshellTerminalSessionsByConn[connId] = normalized;
    queueWebshellPersistStateSave(connId);
}

function getActiveWebshellTerminalSessionId(connId) {
    return getWebshellTerminalSessions(connId).activeId;
}

function getWebshellTerminalLog(connId) {
    if (!connId) return '';
    if (typeof webshellTerminalLogsByConn[connId] === 'string') return webshellTerminalLogsByConn[connId];
    webshellTerminalLogsByConn[connId] = '';
    return '';
}

function saveWebshellTerminalLog(connId, content) {
    if (!connId) return;
    var text = String(content || '');
    var maxLen = 50000; // keep recent terminal output only
    if (text.length > maxLen) text = text.slice(text.length - maxLen);
    webshellTerminalLogsByConn[connId] = text;
}

function appendWebshellTerminalLog(connId, chunk) {
    if (!connId || !chunk) return;
    var current = getWebshellTerminalLog(connId);
    saveWebshellTerminalLog(connId, current + String(chunk));
}

function clearWebshellTerminalLog(connId) {
    if (!connId) return;
    webshellTerminalLogsByConn[connId] = '';
}

function buildWebshellPersistState(connId) {
    var dbState = getWebshellDbState({ id: connId });
    var terminalSessions = getWebshellTerminalSessions(connId);
    return {
        dbState: dbState || null,
        terminalSessions: terminalSessions || null
    };
}

function applyWebshellPersistState(connId, state) {
    if (!connId || !state || typeof state !== 'object') return;
    if (state.dbState && typeof state.dbState === 'object') {
        var key = getWebshellDbStateStorageKey({ id: connId });
        webshellDbConfigByConn[key] = normalizeWebshellDbState(state.dbState);
    }
    if (state.terminalSessions && typeof state.terminalSessions === 'object') {
        webshellTerminalSessionsByConn[connId] = normalizeWebshellTerminalSessions(state.terminalSessions);
    }
}

function queueWebshellPersistStateSave(connId) {
    if (!connId || typeof apiFetch !== 'function') return;
    if (webshellPersistSaveTimersByConn[connId]) clearTimeout(webshellPersistSaveTimersByConn[connId]);
    webshellPersistSaveTimersByConn[connId] = setTimeout(function () {
        delete webshellPersistSaveTimersByConn[connId];
        var payload = buildWebshellPersistState(connId);
        apiFetch('/api/webshell/connections/' + encodeURIComponent(connId) + '/state', {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ state: payload })
        }).catch(function () {});
    }, 500);
}

function ensureWebshellPersistStateLoaded(conn) {
    if (!conn || !conn.id || typeof apiFetch !== 'function') return Promise.resolve();
    if (webshellPersistLoadedByConn[conn.id]) return Promise.resolve();
    return apiFetch('/api/webshell/connections/' + encodeURIComponent(conn.id) + '/state', { method: 'GET' })
        .then(function (r) { return r.ok ? r.json() : Promise.reject(new Error('load state failed')); })
        .then(function (data) {
            applyWebshellPersistState(conn.id, data && data.state ? data.state : {});
            webshellPersistLoadedByConn[conn.id] = true;
        })
        .catch(function () {
            webshellPersistLoadedByConn[conn.id] = true;
        });
}

function setWebshellTerminalStatus(running) {
    webshellTerminalRunning = !!running;
    var el = document.getElementById('webshell-terminal-status');
    if (!el) return;
    el.classList.toggle('running', !!running);
    el.classList.toggle('idle', !running);
    el.textContent = running ? (wsT('webshell.terminalRunning') || '') : (wsT('webshell.terminalIdle') || '');
}

function renderWebshellTerminalSessions(conn) {
    if (!conn || !conn.id) return;
    var tabsEl = document.getElementById('webshell-terminal-sessions');
    if (!tabsEl) return;
    var connId = conn.id;
    var state = getWebshellTerminalSessions(connId);
    var html = '';
    state.sessions.forEach(function (s) {
        var active = s.id === state.activeId;
        html += '<div class="webshell-terminal-session' + (active ? ' active' : '') + '">' +
            '<button type="button" class="webshell-terminal-session-main" data-action="switch" data-terminal-id="' + escapeHtml(s.id) + '">' + escapeHtml(s.name) + '</button>' +
            '<button type="button" class="webshell-terminal-session-close" data-action="close" data-terminal-id="' + escapeHtml(s.id) + '" title="' + escapeHtml(wsT('common.close') || '') + '">×</button>' +
            '</div>';
    });
    html += '<button type="button" class="webshell-terminal-session-add" data-action="add" title="' + escapeHtml(wsT('webshell.terminalNewWindow') || '') + '">+</button>';
    tabsEl.innerHTML = html;
    tabsEl.querySelectorAll('[data-action]').forEach(function (btn) {
        btn.addEventListener('click', function () {
            var action = btn.getAttribute('data-action') || '';
            var targetId = btn.getAttribute('data-terminal-id') || '';
            if (webshellRunning || webshellTerminalRunning) return;
            if (action === 'add') {
                var nextState = getWebshellTerminalSessions(connId);
                var seq = nextState.sessions.length + 1;
                var nextId = 't' + Date.now().toString(36) + Math.random().toString(36).slice(2, 6);
                var prefix = wsT('webshell.terminalWindowPrefix') || '';
                nextState.sessions.push({ id: nextId, name: prefix + seq });
                nextState.activeId = nextId;
                saveWebshellTerminalSessions(connId, nextState);
                destroyWebshellTerminal();
                initWebshellTerminal(conn);
                renderWebshellTerminalSessions(conn);
                return;
            }
            if (!targetId) return;
            if (action === 'close') {
                var curr2 = getWebshellTerminalSessions(connId);
                if (curr2.sessions.length <= 1) return;
                var idx2 = curr2.sessions.findIndex(function (s) { return s.id === targetId; });
                if (idx2 < 0) return;
                curr2.sessions.splice(idx2, 1);
                if (curr2.activeId === targetId) {
                    var fallback = curr2.sessions[Math.max(0, idx2 - 1)] || curr2.sessions[0];
                    curr2.activeId = fallback.id;
                }
                saveWebshellTerminalSessions(connId, curr2);
                // English note.
                var terminalKey = getWebshellTerminalSessionKey(connId, targetId);
                clearWebshellTerminalLog(terminalKey);
                delete webshellHistoryByConn[terminalKey];
                destroyWebshellTerminal();
                initWebshellTerminal(conn);
                renderWebshellTerminalSessions(conn);
                return;
            }
            if (targetId === getActiveWebshellTerminalSessionId(connId)) return;
            var curr = getWebshellTerminalSessions(connId);
            curr.activeId = targetId;
            saveWebshellTerminalSessions(connId, curr);
            destroyWebshellTerminal();
            initWebshellTerminal(conn);
            renderWebshellTerminalSessions(conn);
        });
    });
}

function getWebshellTreeState(conn) {
    var key = safeConnIdForStorage(conn);
    if (!key) return null;
    if (!webshellDirTreeByConn[key]) webshellDirTreeByConn[key] = { '.': [] };
    if (!webshellDirExpandedByConn[key]) webshellDirExpandedByConn[key] = { '.': true };
    if (!webshellDirLoadedByConn[key]) webshellDirLoadedByConn[key] = { '.': false };
    return {
        key: key,
        tree: webshellDirTreeByConn[key],
        expanded: webshellDirExpandedByConn[key],
        loaded: webshellDirLoadedByConn[key]
    };
}

function newWebshellDbProfile(name) {
    var now = Date.now().toString(36);
    var rand = Math.random().toString(36).slice(2, 8);
    return {
        id: 'dbp_' + now + rand,
        name: name || 'DB-1',
        type: 'mysql',
        host: '127.0.0.1',
        port: '3306',
        username: 'root',
        password: '',
        database: '',
        selectedDatabase: '',
        sqlitePath: '/tmp/test.db',
        sql: 'SELECT 1;',
        output: '',
        outputIsError: false,
        schema: {}
    };
}

function getWebshellDbStateStorageKey(conn) {
    return 'webshell_db_state_' + safeConnIdForStorage(conn);
}

function normalizeWebshellDbState(rawState) {
    var state = rawState && typeof rawState === 'object' ? rawState : {};
    var profiles = Array.isArray(state.profiles) ? state.profiles.slice() : [];
    if (!profiles.length) profiles = [newWebshellDbProfile('DB-1')];
    profiles = profiles.map(function (p, idx) {
        var base = newWebshellDbProfile('DB-' + (idx + 1));
        return Object.assign(base, p || {});
    });
    var activeProfileId = state.activeProfileId || '';
    if (!profiles.some(function (p) { return p.id === activeProfileId; })) {
        activeProfileId = profiles[0].id;
    }
    var aiMemo = typeof state.aiMemo === 'string' ? state.aiMemo : '';
    if (aiMemo.length > 100000) aiMemo = aiMemo.slice(0, 100000);
    return { profiles: profiles, activeProfileId: activeProfileId, aiMemo: aiMemo };
}

function getWebshellDbState(conn) {
    var key = getWebshellDbStateStorageKey(conn);
    if (!key) return normalizeWebshellDbState(null);
    if (webshellDbConfigByConn[key]) return webshellDbConfigByConn[key];
    var state = normalizeWebshellDbState(null);
    webshellDbConfigByConn[key] = state;
    return state;
}

function saveWebshellDbState(conn, state) {
    var key = getWebshellDbStateStorageKey(conn);
    if (!key || !state) return;
    var normalized = normalizeWebshellDbState(state);
    webshellDbConfigByConn[key] = normalized;
    if (conn && conn.id) queueWebshellPersistStateSave(conn.id);
}

function getWebshellDbConfig(conn) {
    var state = getWebshellDbState(conn);
    var active = state.profiles.find(function (p) { return p.id === state.activeProfileId; });
    return active || state.profiles[0];
}

function saveWebshellDbConfig(conn, cfg) {
    if (!cfg) return;
    var state = getWebshellDbState(conn);
    var idx = state.profiles.findIndex(function (p) { return p.id === state.activeProfileId; });
    if (idx < 0) idx = 0;
    state.profiles[idx] = Object.assign({}, state.profiles[idx], cfg);
    state.activeProfileId = state.profiles[idx].id;
    saveWebshellDbState(conn, state);
}

function getWebshellAiMemo(conn) {
    var state = getWebshellDbState(conn);
    return typeof state.aiMemo === 'string' ? state.aiMemo : '';
}

function saveWebshellAiMemo(conn, text) {
    var state = getWebshellDbState(conn);
    state.aiMemo = String(text || '');
    if (state.aiMemo.length > 100000) state.aiMemo = state.aiMemo.slice(0, 100000);
    saveWebshellDbState(conn, state);
}

function webshellDbGetFieldValue(id) {
    var el = document.getElementById(id);
    return el && typeof el.value === 'string' ? el.value.trim() : '';
}

function webshellDbCollectConfig(conn) {
    var curr = getWebshellDbConfig(conn) || {};
    var nameVal = webshellDbGetFieldValue('webshell-db-profile-name');
    var cfg = {
        name: nameVal || curr.name || 'DB-1',
        type: webshellDbGetFieldValue('webshell-db-type') || 'mysql',
        host: webshellDbGetFieldValue('webshell-db-host') || '127.0.0.1',
        port: webshellDbGetFieldValue('webshell-db-port') || '',
        username: webshellDbGetFieldValue('webshell-db-user') || '',
        password: (document.getElementById('webshell-db-pass') || {}).value || '',
        database: webshellDbGetFieldValue('webshell-db-name') || '',
        selectedDatabase: curr.selectedDatabase || '',
        sqlitePath: webshellDbGetFieldValue('webshell-db-sqlite-path') || '/tmp/test.db',
        sql: (document.getElementById('webshell-db-sql') || {}).value || ''
    };
    saveWebshellDbConfig(conn, cfg);
    return cfg;
}

function webshellDbUpdateFieldVisibility() {
    var type = webshellDbGetFieldValue('webshell-db-type') || 'mysql';
    var isSqlite = type === 'sqlite';
    var blocks = document.querySelectorAll('.webshell-db-common-field');
    blocks.forEach(function (el) { el.style.display = isSqlite ? 'none' : ''; });
    var sqliteBlock = document.getElementById('webshell-db-sqlite-row');
    if (sqliteBlock) sqliteBlock.style.display = isSqlite ? '' : 'none';
    var portEl = document.getElementById('webshell-db-port');
    if (portEl && !String(portEl.value || '').trim()) {
        if (type === 'mysql') portEl.value = '3306';
        else if (type === 'pgsql') portEl.value = '5432';
        else if (type === 'mssql') portEl.value = '1433';
    }
}

function webshellDbSetOutput(text, isError) {
    var outputEl = document.getElementById('webshell-db-output');
    if (!outputEl) return;
    outputEl.textContent = text || '';
    outputEl.classList.toggle('error', !!isError);
}

function webshellDbRenderTable(rawOutput) {
    var wrap = document.getElementById('webshell-db-result-table');
    if (!wrap) return false;
    var raw = String(rawOutput || '').trim();
    if (!raw) {
        wrap.innerHTML = '';
        return false;
    }
    var lines = raw.split(/\r?\n/).filter(function (line) {
        var t = String(line || '').trim();
        if (!t) return false;
        if (/^\(\d+\s+rows?\)$/i.test(t)) return false;
        if (/^-{3,}$/.test(t)) return false;
        return true;
    });
    if (lines.length < 2) {
        wrap.innerHTML = '';
        return false;
    }
    var delimiter = lines[0].indexOf('\t') >= 0 ? '\t' : (lines[0].indexOf('|') >= 0 ? '|' : '');
    if (!delimiter) {
        wrap.innerHTML = '';
        return false;
    }
    var header = lines[0].split(delimiter).map(function (s) { return String(s || '').trim(); });
    if (!header.length || (header.length === 1 && !header[0])) {
        wrap.innerHTML = '';
        return false;
    }
    var rows = [];
    for (var i = 1; i < lines.length; i++) {
        var line = lines[i];
        if (/^[-+\s|]+$/.test(line)) continue;
        var cols = line.split(delimiter).map(function (s) { return String(s || '').trim(); });
        if (cols.length !== header.length) continue;
        rows.push(cols);
    }
    if (!rows.length) {
        wrap.innerHTML = '';
        return false;
    }
    var maxRows = Math.min(rows.length, 200);
    var html = '<table class="webshell-db-table"><thead><tr>';
    header.forEach(function (h) { html += '<th>' + escapeHtml(h || '-') + '</th>'; });
    html += '</tr></thead><tbody>';
    for (var r = 0; r < maxRows; r++) {
        html += '<tr>';
        rows[r].forEach(function (c) { html += '<td>' + escapeHtml(c || '') + '</td>'; });
        html += '</tr>';
    }
    html += '</tbody></table>';
    if (rows.length > maxRows) {
        html += '<div class="webshell-db-table-meta"> ' + maxRows + ' ， ' + rows.length + ' </div>';
    } else {
        html += '<div class="webshell-db-table-meta"> ' + rows.length + ' ，' + header.length + ' </div>';
    }
    wrap.innerHTML = html;
    return true;
}

function webshellDbQuoteIdentifier(type, name) {
    var v = String(name || '');
    if (!v) return '';
    if (type === 'mysql') return '`' + v.replace(/`/g, '``') + '`';
    if (type === 'mssql') return '[' + v.replace(/]/g, ']]') + ']';
    return '"' + v.replace(/"/g, '""') + '"';
}

function webshellDbQuoteLiteral(value) {
    return "'" + String(value == null ? '' : value).replace(/'/g, "''") + "'";
}

function buildWebshellDbCommand(cfg, isTestOnly, options) {
    options = options || {};
    var type = cfg.type || 'mysql';
    var sql = String(isTestOnly ? 'SELECT 1;' : (options.sql || cfg.sql || '')).trim();
    if (!sql) return { error: wsT('webshell.dbSqlRequired') || ' SQL' };

    var sqlB64 = btoa(unescape(encodeURIComponent(sql)));
    var sqlB64Arg = escapeSingleQuotedShellArg(sqlB64);
    var tmpFile = '/tmp/.csai_sql_$$.sql';
    var decodeToFile = 'printf %s ' + sqlB64Arg + " | base64 -d > " + tmpFile;
    var cleanup = '; rc=$?; rm -f ' + tmpFile + '; echo "__CSAI_DB_RC__:$rc"; exit $rc';
    var command = '';

    if (type === 'mysql') {
        var host = escapeSingleQuotedShellArg(cfg.host || '127.0.0.1');
        var port = escapeSingleQuotedShellArg(cfg.port || '3306');
        var user = escapeSingleQuotedShellArg(cfg.username || 'root');
        var pass = escapeSingleQuotedShellArg(cfg.password || '');
        var dbName = cfg.selectedDatabase || cfg.database || '';
        var db = dbName ? (' -D ' + escapeSingleQuotedShellArg(dbName)) : '';
        command = decodeToFile + '; MYSQL_PWD=' + pass + ' mysql -h ' + host + ' -P ' + port + ' -u ' + user + db + ' --batch --raw < ' + tmpFile + cleanup;
    } else if (type === 'pgsql') {
        var pHost = escapeSingleQuotedShellArg(cfg.host || '127.0.0.1');
        var pPort = escapeSingleQuotedShellArg(cfg.port || '5432');
        var pUser = escapeSingleQuotedShellArg(cfg.username || 'postgres');
        var pPass = escapeSingleQuotedShellArg(cfg.password || '');
        var pDb = escapeSingleQuotedShellArg(cfg.selectedDatabase || cfg.database || 'postgres');
        command = decodeToFile + '; PGPASSWORD=' + pPass + ' psql -h ' + pHost + ' -p ' + pPort + ' -U ' + pUser + ' -d ' + pDb + ' -v ON_ERROR_STOP=1 -A -F "|" -P footer=off -f ' + tmpFile + cleanup;
    } else if (type === 'sqlite') {
        var sqlitePath = escapeSingleQuotedShellArg(cfg.sqlitePath || '/tmp/test.db');
        command = decodeToFile + '; sqlite3 -header -separator "|" ' + sqlitePath + ' < ' + tmpFile + cleanup;
    } else if (type === 'mssql') {
        var sHost = cfg.host || '127.0.0.1';
        var sPort = cfg.port || '1433';
        var sUser = escapeSingleQuotedShellArg(cfg.username || 'sa');
        var sPass = escapeSingleQuotedShellArg(cfg.password || '');
        var sDb = escapeSingleQuotedShellArg(cfg.selectedDatabase || cfg.database || 'master');
        var server = escapeSingleQuotedShellArg(sHost + ',' + sPort);
        command = decodeToFile + '; sqlcmd -S ' + server + ' -U ' + sUser + ' -P ' + sPass + ' -W -s "|" -d ' + sDb + ' -i ' + tmpFile + cleanup;
    } else {
        return { error: (wsT('webshell.dbExecFailed') || '') + ': unsupported type ' + type };
    }

    return { command: command };
}

function buildWebshellDbSchemaCommand(cfg) {
    var type = cfg.type || 'mysql';
    var schemaSQL = '';
    if (type === 'mysql') {
        schemaSQL = "SELECT SCHEMA_NAME AS db_name, '' AS table_name, '' AS column_name FROM INFORMATION_SCHEMA.SCHEMATA UNION ALL SELECT TABLE_SCHEMA AS db_name, TABLE_NAME AS table_name, '' AS column_name FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE='BASE TABLE' UNION ALL SELECT TABLE_SCHEMA AS db_name, TABLE_NAME AS table_name, COLUMN_NAME AS column_name FROM INFORMATION_SCHEMA.COLUMNS ORDER BY db_name, table_name, column_name;";
    } else if (type === 'pgsql') {
        schemaSQL = "SELECT table_schema AS db_name, table_name, '' AS column_name FROM information_schema.tables WHERE table_type='BASE TABLE' AND table_schema NOT IN ('pg_catalog','information_schema') UNION ALL SELECT table_schema AS db_name, table_name, column_name FROM information_schema.columns WHERE table_schema NOT IN ('pg_catalog','information_schema') ORDER BY db_name, table_name, column_name;";
    } else if (type === 'sqlite') {
        schemaSQL = "SELECT 'main' AS db_name, name AS table_name, '' AS column_name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' UNION ALL SELECT 'main' AS db_name, m.name AS table_name, p.name AS column_name FROM sqlite_master m JOIN pragma_table_info(m.name) p ON 1=1 WHERE m.type='table' AND m.name NOT LIKE 'sqlite_%' ORDER BY db_name, table_name, column_name;";
    } else if (type === 'mssql') {
        schemaSQL = "SELECT TABLE_SCHEMA AS db_name, TABLE_NAME AS table_name, '' AS column_name FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE='BASE TABLE' UNION ALL SELECT TABLE_SCHEMA AS db_name, TABLE_NAME AS table_name, COLUMN_NAME AS column_name FROM INFORMATION_SCHEMA.COLUMNS ORDER BY db_name, table_name, column_name;";
    } else {
        return { error: (wsT('webshell.dbExecFailed') || '') + ': unsupported type ' + type };
    }
    return buildWebshellDbCommand(cfg, false, { sql: schemaSQL });
}

function buildWebshellDbColumnsCommand(cfg, dbName, tableName) {
    var type = cfg.type || 'mysql';
    var sql = '';
    if (type === 'mysql') {
        sql = "SELECT COLUMN_NAME AS column_name FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA=" + webshellDbQuoteLiteral(dbName) + " AND TABLE_NAME=" + webshellDbQuoteLiteral(tableName) + " ORDER BY ORDINAL_POSITION;";
    } else if (type === 'pgsql') {
        sql = "SELECT column_name FROM information_schema.columns WHERE table_schema=" + webshellDbQuoteLiteral(dbName) + " AND table_name=" + webshellDbQuoteLiteral(tableName) + " ORDER BY ordinal_position;";
    } else if (type === 'sqlite') {
        sql = "SELECT name AS column_name FROM pragma_table_info(" + webshellDbQuoteLiteral(tableName) + ") ORDER BY cid;";
    } else if (type === 'mssql') {
        sql = "SELECT COLUMN_NAME AS column_name FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA=" + webshellDbQuoteLiteral(dbName) + " AND TABLE_NAME=" + webshellDbQuoteLiteral(tableName) + " ORDER BY ORDINAL_POSITION;";
    } else {
        return { error: (wsT('webshell.dbExecFailed') || '') + ': unsupported type ' + type };
    }
    return buildWebshellDbCommand(cfg, false, { sql: sql });
}

function buildWebshellDbColumnsByDatabaseCommand(cfg, dbName) {
    var type = cfg.type || 'mysql';
    var sql = '';
    if (type === 'mysql') {
        sql = "SELECT TABLE_NAME AS table_name, COLUMN_NAME AS column_name FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA=" + webshellDbQuoteLiteral(dbName) + " ORDER BY TABLE_NAME, ORDINAL_POSITION;";
    } else if (type === 'pgsql') {
        sql = "SELECT table_name, column_name FROM information_schema.columns WHERE table_schema=" + webshellDbQuoteLiteral(dbName) + " ORDER BY table_name, ordinal_position;";
    } else if (type === 'sqlite') {
        sql = "SELECT m.name AS table_name, p.name AS column_name FROM sqlite_master m JOIN pragma_table_info(m.name) p ON 1=1 WHERE m.type='table' AND m.name NOT LIKE 'sqlite_%' ORDER BY m.name, p.cid;";
    } else if (type === 'mssql') {
        sql = "SELECT TABLE_NAME AS table_name, COLUMN_NAME AS column_name FROM INFORMATION_SCHEMA.COLUMNS WHERE TABLE_SCHEMA=" + webshellDbQuoteLiteral(dbName) + " ORDER BY TABLE_NAME, ORDINAL_POSITION;";
    } else {
        return { error: (wsT('webshell.dbExecFailed') || '') + ': unsupported type ' + type };
    }
    return buildWebshellDbCommand(cfg, false, { sql: sql });
}

function parseWebshellDbExecOutput(rawOutput) {
    var raw = String(rawOutput || '');
    var rc = null;
    var cleaned = raw.replace(/__CSAI_DB_RC__:(\d+)\s*$/m, function (_, code) {
        rc = parseInt(code, 10);
        return '';
    }).trim();
    return { rc: rc, output: cleaned };
}

function parseWebshellDbSchema(rawOutput) {
    var text = String(rawOutput || '').trim();
    if (!text) return {};
    var lines = text.split(/\r?\n/).filter(function (line) {
        return line && line.trim() && !/^\(\d+\s+rows?\)$/i.test(line.trim()) && !/^[-+\s|]+$/.test(line.trim());
    });
    if (lines.length < 2) return {};
    var delimiter = lines[0].indexOf('\t') >= 0 ? '\t' : (lines[0].indexOf('|') >= 0 ? '|' : '');
    if (!delimiter) return {};
    var headers = lines[0].split(delimiter).map(function (s) { return String(s || '').trim().toLowerCase(); });
    var dbIdx = headers.indexOf('db_name');
    var tableIdx = headers.indexOf('table_name');
    var columnIdx = headers.indexOf('column_name');
    if (dbIdx < 0 || tableIdx < 0) return {};
    var schema = {};
    for (var i = 1; i < lines.length; i++) {
        var cols = lines[i].split(delimiter).map(function (s) { return String(s || '').trim(); });
        if (cols.length !== headers.length) continue;
        var db = cols[dbIdx] || 'default';
        var table = cols[tableIdx] || '';
        var column = columnIdx >= 0 ? (cols[columnIdx] || '') : '';
        if (!schema[db]) schema[db] = { tables: {} };
        if (!table) continue;
        if (!schema[db].tables[table]) schema[db].tables[table] = [];
        if (column && schema[db].tables[table].indexOf(column) < 0) {
            schema[db].tables[table].push(column);
        }
    }
    return normalizeWebshellDbSchema(schema);
}

function parseWebshellDbColumns(rawOutput) {
    var text = String(rawOutput || '').trim();
    if (!text) return [];
    var lines = text.split(/\r?\n/).filter(function (line) {
        return line && line.trim() && !/^\(\d+\s+rows?\)$/i.test(line.trim()) && !/^[-+\s|]+$/.test(line.trim());
    });
    if (lines.length < 2) return [];
    var delimiter = lines[0].indexOf('\t') >= 0 ? '\t' : (lines[0].indexOf('|') >= 0 ? '|' : '');
    if (!delimiter) {
        if (String(lines[0] || '').trim().toLowerCase() !== 'column_name') return [];
        var plainColumns = [];
        for (var p = 1; p < lines.length; p++) {
            var plainName = String(lines[p] || '').trim();
            if (!plainName || plainColumns.indexOf(plainName) >= 0) continue;
            plainColumns.push(plainName);
        }
        return plainColumns;
    }
    var headers = lines[0].split(delimiter).map(function (s) { return String(s || '').trim().toLowerCase(); });
    var colIdx = headers.indexOf('column_name');
    if (colIdx < 0) return [];
    var columns = [];
    for (var i = 1; i < lines.length; i++) {
        var cols = lines[i].split(delimiter).map(function (s) { return String(s || '').trim(); });
        if (cols.length !== headers.length) continue;
        var name = cols[colIdx] || '';
        if (!name || columns.indexOf(name) >= 0) continue;
        columns.push(name);
    }
    return columns;
}

function parseWebshellDbTableColumns(rawOutput) {
    var text = String(rawOutput || '').trim();
    if (!text) return {};
    var lines = text.split(/\r?\n/).filter(function (line) {
        return line && line.trim() && !/^\(\d+\s+rows?\)$/i.test(line.trim()) && !/^[-+\s|]+$/.test(line.trim());
    });
    if (lines.length < 2) return {};
    var delimiter = lines[0].indexOf('\t') >= 0 ? '\t' : (lines[0].indexOf('|') >= 0 ? '|' : '');
    if (!delimiter) return {};
    var headers = lines[0].split(delimiter).map(function (s) { return String(s || '').trim().toLowerCase(); });
    var tableIdx = headers.indexOf('table_name');
    var colIdx = headers.indexOf('column_name');
    if (tableIdx < 0 || colIdx < 0) return {};
    var tableColumns = {};
    for (var i = 1; i < lines.length; i++) {
        var cols = lines[i].split(delimiter).map(function (s) { return String(s || '').trim(); });
        if (cols.length !== headers.length) continue;
        var tableName = cols[tableIdx] || '';
        var colName = cols[colIdx] || '';
        if (!tableName || !colName) continue;
        if (!tableColumns[tableName]) tableColumns[tableName] = [];
        if (tableColumns[tableName].indexOf(colName) >= 0) continue;
        tableColumns[tableName].push(colName);
    }
    return tableColumns;
}

function normalizeWebshellDbSchema(rawSchema) {
    if (!rawSchema || typeof rawSchema !== 'object') return {};
    var normalized = {};
    Object.keys(rawSchema).forEach(function (dbName) {
        var dbEntry = rawSchema[dbName];
        var tableMap = {};

        if (Array.isArray(dbEntry)) {
            dbEntry.forEach(function (tableName) {
                var t = String(tableName || '').trim();
                if (!t) return;
                if (!tableMap[t]) tableMap[t] = [];
            });
        } else if (dbEntry && typeof dbEntry === 'object') {
            var tablesSource = (dbEntry.tables && typeof dbEntry.tables === 'object') ? dbEntry.tables : dbEntry;
            Object.keys(tablesSource).forEach(function (tableName) {
                if (tableName === 'tables') return;
                var t = String(tableName || '').trim();
                if (!t) return;
                var rawColumns = tablesSource[tableName];
                var columns = Array.isArray(rawColumns) ? rawColumns : [];
                var uniqColumns = [];
                columns.forEach(function (colName) {
                    var c = String(colName || '').trim();
                    if (!c || uniqColumns.indexOf(c) >= 0) return;
                    uniqColumns.push(c);
                });
                uniqColumns.sort(function (a, b) { return a.localeCompare(b); });
                tableMap[t] = uniqColumns;
            });
        }

        var sortedTables = {};
        Object.keys(tableMap).sort(function (a, b) { return a.localeCompare(b); }).forEach(function (tableName) {
            sortedTables[tableName] = tableMap[tableName];
        });
        normalized[dbName] = { tables: sortedTables };
    });
    return normalized;
}

function simplifyWebshellAiError(rawMessage) {
    var msg = String(rawMessage || '').trim();
    var lower = msg.toLowerCase();
    if ((lower.indexOf('401') !== -1 || lower.indexOf('unauthorized') !== -1) &&
        (lower.indexOf('api key') !== -1 || lower.indexOf('apikey') !== -1)) {
        return '：API Key （401）';
    }
    if (lower.indexOf('timeout') !== -1 || lower.indexOf('timed out') !== -1) {
        return '，';
    }
    if (lower.indexOf('network') !== -1 || lower.indexOf('failed to fetch') !== -1) {
        return '，';
    }
    return msg || '';
}

function renderWebshellAiErrorMessage(targetEl, rawMessage) {
    if (!targetEl) return;
    var full = String(rawMessage || '').trim();
    var shortMsg = simplifyWebshellAiError(full);
    targetEl.classList.add('webshell-ai-msg-error');
    targetEl.innerHTML = '';
    var head = document.createElement('div');
    head.className = 'webshell-ai-error-head';
    head.textContent = shortMsg;
    targetEl.appendChild(head);
    if (full && full !== shortMsg) {
        var detail = document.createElement('details');
        detail.className = 'webshell-ai-error-detail';
        var summary = document.createElement('summary');
        summary.textContent = '';
        var pre = document.createElement('pre');
        pre.textContent = full;
        detail.appendChild(summary);
        detail.appendChild(pre);
        targetEl.appendChild(detail);
    }
}

function isLikelyWebshellAiErrorMessage(content, msg) {
    var text = String(content || '').trim();
    if (!text) return false;
    var lower = text.toLowerCase();
    if (/^(|||error)\s*[:：]/i.test(text)) return true;
    if (/(status code\s*:\s*4\d{2}|unauthorized|forbidden|apikey|api key|invalid api key)/i.test(lower)) return true;
    if (/(noderunerror|tool[-_ ]?error|agent[-_ ]?error|)/i.test(lower)) return true;
    var details = msg && Array.isArray(msg.processDetails) ? msg.processDetails : [];
    return details.some(function (d) { return String((d && d.eventType) || '').toLowerCase() === 'error'; });
}

function formatWebshellAiConvDate(updatedAt) {
    if (!updatedAt) return '';
    var d = typeof updatedAt === 'string' ? new Date(updatedAt) : updatedAt;
    if (isNaN(d.getTime())) return '';
    var now = new Date();
    var sameDay = d.getDate() === now.getDate() && d.getMonth() === now.getMonth() && d.getFullYear() === now.getFullYear();
    if (sameDay) return d.getHours() + ':' + String(d.getMinutes()).padStart(2, '0');
    return (d.getMonth() + 1) + '/' + d.getDate();
}

function webshellAgentPx(data) {
    if (!data || data.einoAgent == null) return '';
    var s = String(data.einoAgent).trim();
    return s ? ('[' + s + '] ') : '';
}

// English note.
function buildWebshellTimelineItemFromDetail(detail) {
    var eventType = detail.eventType || '';
    var title = detail.message || '';
    var data = detail.data || {};
    var ap = webshellAgentPx(data);
    if (eventType === 'iteration') {
        title = ap + ((typeof window.t === 'function') ? window.t('chat.iterationRound', { n: data.iteration || 1 }) : (' ' + (data.iteration || 1) + ' '));
    } else if (eventType === 'thinking') {
        title = ap + '🤔 ' + ((typeof window.t === 'function') ? window.t('chat.aiThinking') : 'AI ');
    } else if (eventType === 'tool_calls_detected') {
        title = ap + '🔧 ' + ((typeof window.t === 'function') ? window.t('chat.toolCallsDetected', { count: data.count || 0 }) : (' ' + (data.count || 0) + ' '));
    } else if (eventType === 'tool_call') {
        var tn = data.toolName || ((typeof window.t === 'function') ? window.t('chat.unknownTool') : '');
        var idx = data.index || 0;
        var total = data.total || 0;
        title = ap + '🔧 ' + ((typeof window.t === 'function') ? window.t('chat.callTool', { name: tn, index: idx, total: total }) : (': ' + tn + (total ? ' (' + idx + '/' + total + ')' : '')));
    } else if (eventType === 'tool_result') {
        var success = data.success !== false;
        var tname = data.toolName || '';
        title = ap + (success ? '✅ ' : '❌ ') + ((typeof window.t === 'function') ? (success ? window.t('chat.toolExecComplete', { name: tname }) : window.t('chat.toolExecFailed', { name: tname })) : (tname + (success ? ' ' : ' ')));
    } else if (eventType === 'eino_agent_reply') {
        title = ap + '💬 ' + ((typeof window.t === 'function') ? window.t('chat.einoAgentReplyTitle') : '');
    } else if (eventType === 'progress') {
        title = (typeof window.translateProgressMessage === 'function') ? window.translateProgressMessage(detail.message || '') : (detail.message || '');
    }
    var html = '<span class="webshell-ai-timeline-title">' + escapeHtml(title || '') + '</span>';
    if (eventType === 'eino_agent_reply' && detail.message) {
        html += '<div class="webshell-ai-timeline-msg"><pre style="white-space:pre-wrap;">' + escapeHtml(detail.message) + '</pre></div>';
    }
    if (eventType === 'tool_call' && data && (data.argumentsObj || data.arguments)) {
        try {
            var args = data.argumentsObj;
            if (args == null && data.arguments != null && String(data.arguments).trim() !== '') {
                try {
                    args = JSON.parse(String(data.arguments));
                } catch (e2) {
                    args = { _raw: String(data.arguments) };
                }
            }
            if (args && typeof args === 'object') {
                var paramsLabel = (typeof window.t === 'function') ? window.t('timeline.params') : ':';
                html += '<div class="webshell-ai-timeline-msg"><div class="tool-arg-section"><strong>' + escapeHtml(paramsLabel) + '</strong><pre class="tool-args">' + escapeHtml(JSON.stringify(args, null, 2)) + '</pre></div></div>';
            }
        } catch (e) {}
    } else if (eventType === 'tool_result' && data) {
        var isError = data.isError || data.success === false;
        var noResultText = (typeof window.t === 'function') ? window.t('timeline.noResult') : '';
        var result = data.result != null ? data.result : (data.error != null ? data.error : noResultText);
        var resultStr = (typeof result === 'string') ? result : JSON.stringify(result);
        var execResultLabel = (typeof window.t === 'function') ? window.t('timeline.executionResult') : ':';
        var execIdLabel = (typeof window.t === 'function') ? window.t('timeline.executionId') : 'ID:';
        html += '<div class="webshell-ai-timeline-msg"><div class="tool-result-section ' + (isError ? 'error' : 'success') + '"><strong>' + escapeHtml(execResultLabel) + '</strong><pre class="tool-result">' + escapeHtml(resultStr) + '</pre>' + (data.executionId ? '<div class="tool-execution-id"><span>' + escapeHtml(execIdLabel) + '</span> <code>' + escapeHtml(String(data.executionId)) + '</code></div>' : '') + '</div></div>';
    } else if (detail.message && detail.message !== title) {
        html += '<div class="webshell-ai-timeline-msg">' + escapeHtml(detail.message) + '</div>';
    }
    return html;
}

// English note.
function renderWebshellProcessDetailsBlock(processDetails, defaultCollapsed) {
    if (!processDetails || processDetails.length === 0) return null;
    var expandLabel = (typeof window.t === 'function') ? window.t('chat.expandDetail') : '';
    var collapseLabel = (typeof window.t === 'function') ? window.t('tasks.collapseDetail') : '';
    var headerLabel = (typeof window.t === 'function') ? (window.t('chat.penetrationTestDetail') || '') : '';
    var wrapper = document.createElement('div');
    wrapper.className = 'process-details-container webshell-ai-process-block';
    var collapsed = defaultCollapsed !== false;
    wrapper.innerHTML = '<button type="button" class="webshell-ai-process-toggle" aria-expanded="' + (!collapsed) + '">' + escapeHtml(headerLabel) + ' <span class="ws-toggle-icon">' + (collapsed ? '▶' : '▼') + '</span></button><div class="process-details-content"><div class="progress-timeline webshell-ai-timeline has-items' + (collapsed ? '' : ' expanded') + '"></div></div>';
    var timeline = wrapper.querySelector('.progress-timeline');
    processDetails.forEach(function (d) {
        var item = document.createElement('div');
        item.className = 'webshell-ai-timeline-item webshell-ai-timeline-' + (d.eventType || '');
        item.innerHTML = buildWebshellTimelineItemFromDetail(d);
        timeline.appendChild(item);
    });
    var toggleBtn = wrapper.querySelector('.webshell-ai-process-toggle');
    var toggleIcon = wrapper.querySelector('.ws-toggle-icon');
    toggleBtn.addEventListener('click', function () {
        var isExpanded = timeline.classList.contains('expanded');
        timeline.classList.toggle('expanded');
        toggleBtn.setAttribute('aria-expanded', !isExpanded);
        if (toggleIcon) toggleIcon.textContent = isExpanded ? '▶' : '▼';
    });
    return wrapper;
}

function fetchAndRenderWebshellAiConvList(conn, listEl) {
    if (!conn || !conn.id || !listEl || typeof apiFetch !== 'function') return Promise.resolve();
    return apiFetch('/api/webshell/connections/' + encodeURIComponent(conn.id) + '/ai-conversations', { method: 'GET' })
        .then(function (r) { return r.json(); })
        .then(function (list) {
            if (!Array.isArray(list)) list = [];
            listEl.innerHTML = '';
            list.forEach(function (item) {
                var row = document.createElement('div');
                row.className = 'webshell-ai-conv-item';
                row.dataset.convId = item.id;
                var title = (item.title || '').trim() || item.id.slice(0, 8);
                var dateStr = item.updatedAt ? formatWebshellAiConvDate(item.updatedAt) : '';
                row.innerHTML = '<span class="webshell-ai-conv-item-title">' + escapeHtml(title) + '</span><span class="webshell-ai-conv-item-date">' + escapeHtml(dateStr) + '</span>';
                if (webshellAiConvMap[conn.id] === item.id) row.classList.add('active');
                row.addEventListener('click', function () {
                    webshellAiConvListSelect(conn, item.id, document.getElementById('webshell-ai-messages'), listEl);
                });
                var delBtn = document.createElement('button');
                delBtn.type = 'button';
                delBtn.className = 'btn-ghost btn-sm webshell-ai-conv-del';
                delBtn.textContent = '×';
                delBtn.title = wsT('webshell.aiDeleteConversation') || '';
                delBtn.addEventListener('click', function (e) {
                    e.stopPropagation();
                    if (!confirm(wsT('webshell.aiDeleteConversationConfirm') || '？')) return;
                    var deletedId = item.id;
                    apiFetch('/api/conversations/' + encodeURIComponent(deletedId), { method: 'DELETE' })
                        .then(function (r) {
                            if (r.ok) {
                                if (webshellAiConvMap[conn.id] === deletedId) {
                                    delete webshellAiConvMap[conn.id];
                                    var msgs = document.getElementById('webshell-ai-messages');
                                    if (msgs) msgs.innerHTML = '';
                                }
                                fetchAndRenderWebshellAiConvList(conn, listEl);
                                try {
                                    document.dispatchEvent(new CustomEvent('conversation-deleted', { detail: { conversationId: deletedId } }));
                                } catch (err) { /* ignore */ }
                            }
                        })
                        .catch(function (e) { console.warn('', e); });
                });
                row.appendChild(delBtn);
                listEl.appendChild(row);
            });
        })
        .catch(function (e) { console.warn('', e); });
}

function webshellAiConvListSelect(conn, convId, messagesContainer, listEl) {
    if (!conn || !convId || !messagesContainer) return;
    webshellAiConvMap[conn.id] = convId;
    if (listEl) listEl.querySelectorAll('.webshell-ai-conv-item').forEach(function (el) {
        el.classList.toggle('active', el.dataset.convId === convId);
    });
    if (typeof apiFetch !== 'function') return;
    apiFetch('/api/conversations/' + encodeURIComponent(convId) + '?include_process_details=1', { method: 'GET' })
        .then(function (r) { return r.json(); })
        .then(function (data) {
            messagesContainer.innerHTML = '';
            var list = data.messages || [];
            list.forEach(function (msg) {
                var role = (msg.role || '').toLowerCase();
                var content = (msg.content || '').trim();
                if (!content && role !== 'assistant') return;
                var div = document.createElement('div');
                div.className = 'webshell-ai-msg ' + (role === 'user' ? 'user' : 'assistant');
                if (role === 'user') {
                    div.textContent = content;
                } else {
                    if (isLikelyWebshellAiErrorMessage(content, msg)) {
                        renderWebshellAiErrorMessage(div, content);
                    } else if (typeof formatMarkdown === 'function') {
                        div.innerHTML = formatMarkdown(content);
                    } else {
                        div.textContent = content;
                    }
                }
                messagesContainer.appendChild(div);
                if (role === 'assistant' && msg.processDetails && msg.processDetails.length > 0) {
                    var block = renderWebshellProcessDetailsBlock(msg.processDetails, true);
                    if (block) messagesContainer.appendChild(block);
                }
            });
            if (list.length === 0) {
                var readyMsg = wsT('webshell.aiSystemReadyMessage') || '。，。';
                var readyDiv = document.createElement('div');
                readyDiv.className = 'webshell-ai-msg assistant';
                readyDiv.textContent = readyMsg;
                messagesContainer.appendChild(readyDiv);
            }
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
        })
        .catch(function (e) { console.warn('', e); });
}

// English note.
function selectWebshell(id, stateReady) {
    currentWebshellId = id;
    renderWebshellList();
    const conn = webshellConnections.find(c => c.id === id);
    const workspace = document.getElementById('webshell-workspace');
    if (!workspace) return;
    if (!conn) {
        workspace.innerHTML = '<div class="webshell-workspace-placeholder">' + wsT('webshell.selectOrAdd') + '</div>';
        return;
    }
    if (!stateReady) {
        ensureWebshellPersistStateLoaded(conn).then(function () {
            if (currentWebshellId === id) selectWebshell(id, true);
        });
        return;
    }

    destroyWebshellTerminal();
    webshellCurrentConn = conn;

    workspace.innerHTML =
        '<div class="webshell-tabs">' +
        '<button type="button" class="webshell-tab active" data-tab="terminal">' + wsT('webshell.tabTerminal') + '</button>' +
        '<button type="button" class="webshell-tab" data-tab="file">' + wsT('webshell.tabFileManager') + '</button>' +
        '<button type="button" class="webshell-tab" data-tab="db">' + (wsT('webshell.tabDbManager') || '') + '</button>' +
        '<button type="button" class="webshell-tab" data-tab="ai">' + (wsT('webshell.tabAiAssistant') || 'AI ') + '</button>' +
        '<button type="button" class="webshell-tab" data-tab="memo">' + (wsT('webshell.tabMemo') || '') + '</button>' +
        '</div>' +
        '<div id="webshell-pane-terminal" class="webshell-pane active">' +
        '<div class="webshell-terminal-toolbar">' +
        '<button type="button" class="btn-ghost btn-sm" id="webshell-terminal-clear" title="' + (wsT('webshell.clearScreen') || '') + '">' + (wsT('webshell.clearScreen') || '') + '</button> ' +
        '<button type="button" class="btn-ghost btn-sm" id="webshell-terminal-copy-log" title="' + (wsT('webshell.copyTerminalLog') || '') + '">' + (wsT('webshell.copyTerminalLog') || '') + '</button> ' +
        '<span id="webshell-terminal-status" class="webshell-terminal-status idle">' + (wsT('webshell.terminalIdle') || '') + '</span> ' +
        '<span class="webshell-quick-label">' + (wsT('webshell.quickCommands') || '') + ':</span> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="whoami">whoami</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="id">id</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="pwd">pwd</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="ls -la">ls -la</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="uname -a">uname -a</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="ifconfig">ifconfig</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="ip a">ip a</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="env">env</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="hostname">hostname</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="ps aux">ps aux</button> ' +
        '<button type="button" class="btn-ghost btn-sm webshell-quick-cmd" data-cmd="netstat -tulnp">netstat</button>' +
        '</div>' +
        '<div class="webshell-terminal-shell">' +
        '<div id="webshell-terminal-sessions" class="webshell-terminal-sessions"></div>' +
        '<div id="webshell-terminal-container" class="webshell-terminal-container"></div>' +
        '</div>' +
        '</div>' +
        '<div id="webshell-pane-file" class="webshell-pane">' +
        '<div class="webshell-file-layout">' +
        '<aside class="webshell-file-sidebar">' +
        '<div class="webshell-file-sidebar-title">' + wsTOr('webshell.dirTree', '') + '</div>' +
        '<div id="webshell-dir-tree" class="webshell-dir-tree"></div>' +
        '</aside>' +
        '<section class="webshell-file-main">' +
        '<div class="webshell-file-toolbar">' +
        '<div class="webshell-file-breadcrumb" id="webshell-file-breadcrumb"></div>' +
        '<div class="webshell-file-toolbar-main">' +
        '<label class="webshell-file-path-field"><span>' + wsT('webshell.filePath') + '</span> <input type="text" id="webshell-file-path" class="form-control" value="." /></label>' +
        '<input type="text" id="webshell-file-filter" class="form-control webshell-file-filter" placeholder="' + (wsT('webshell.filterPlaceholder') || '') + '" />' +
        '<button type="button" class="btn-secondary" id="webshell-list-dir">' + wsT('webshell.listDir') + '</button>' +
        '<button type="button" class="btn-ghost" id="webshell-parent-dir">' + wsT('webshell.parentDir') + '</button>' +
        '</div>' +
        '<div class="webshell-file-toolbar-actions">' +
        '<button type="button" class="btn-ghost" id="webshell-file-refresh" title="' + (wsT('webshell.refresh') || '') + '">' + (wsT('webshell.refresh') || '') + '</button>' +
        '<details class="webshell-toolbar-actions">' +
        '<summary class="btn-ghost webshell-toolbar-actions-btn">' + (wsT('webshell.moreActions') || '') + '</summary>' +
        '<div class="webshell-row-actions-menu">' +
        '<button type="button" class="btn-ghost" id="webshell-mkdir-btn">' + (wsT('webshell.newDir') || '') + '</button>' +
        '<button type="button" class="btn-ghost" id="webshell-newfile-btn">' + (wsT('webshell.newFile') || '') + '</button>' +
        '<button type="button" class="btn-ghost" id="webshell-upload-btn">' + (wsT('webshell.upload') || '') + '</button>' +
        '<button type="button" class="btn-ghost" id="webshell-batch-delete-btn">' + (wsT('webshell.batchDelete') || '') + '</button>' +
        '<button type="button" class="btn-ghost" id="webshell-batch-download-btn">' + (wsT('webshell.batchDownload') || '') + '</button>' +
        '</div></details>' +
        '</div>' +
        '</div>' +
        '<div id="webshell-file-list" class="webshell-file-list"></div>' +
        '</section>' +
        '</div>' +
        '</div>' +
        '<div id="webshell-pane-ai" class="webshell-pane webshell-pane-ai-with-sidebar">' +
        '<div class="webshell-ai-sidebar">' +
        '<button type="button" class="btn-primary btn-sm webshell-ai-new-btn" id="webshell-ai-new-conv">' + (wsT('webshell.aiNewConversation') || '') + '</button>' +
        '<div class="webshell-ai-conv-list" id="webshell-ai-conv-list"></div>' +
        '</div>' +
        '<div class="webshell-ai-main">' +
        '<div id="webshell-ai-messages" class="webshell-ai-messages"></div>' +
        '<div class="webshell-ai-input-area">' +
        '<div class="webshell-ai-selectors-row">' +
        '<div class="ws-role-selector-wrapper">' +
        '<button type="button" class="role-selector-btn ws-role-selector-btn" id="ws-role-selector-btn" onclick="wsToggleRolePanel()">' +
        '<span id="ws-role-selector-icon" class="role-selector-icon">\ud83d\udd35</span>' +
        '<span id="ws-role-selector-text" class="role-selector-text">' + (wsT('chat.defaultRole') || '') + '</span>' +
        '<svg class="role-selector-arrow" width="10" height="10" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M6 9l6 6 6-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>' +
        '</button>' +
        '<div id="ws-role-selection-panel" class="role-selection-panel" style="display:none;">' +
        '<div class="role-selection-panel-header"><h3 class="role-selection-panel-title">' + (wsT('chatGroup.rolePanelTitle') || '') + '</h3>' +
        '<button type="button" class="role-selection-panel-close" onclick="wsCloseRolePanel()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M18 6L6 18M6 6l12 12" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg></button>' +
        '</div><div id="ws-role-selection-list" class="role-selection-list-main"></div></div>' +
        '</div>' +
        '<div class="ws-agent-mode-wrapper" id="ws-agent-mode-wrapper" style="display:none;">' +
        '<div class="agent-mode-inner">' +
        '<button type="button" class="role-selector-btn agent-mode-btn" id="ws-agent-mode-btn" onclick="wsToggleAgentModePanel()">' +
        '<span id="ws-agent-mode-icon" class="role-selector-icon">\ud83e\udd16</span>' +
        '<span id="ws-agent-mode-text" class="role-selector-text">' + (wsT('chat.agentModeReactNative') || ' ReAct') + '</span>' +
        '<svg class="role-selector-arrow" width="10" height="10" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M6 9l6 6 6-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>' +
        '</button>' +
        '<div id="ws-agent-mode-panel" class="agent-mode-panel" style="display:none;" role="listbox">' +
        '<div class="role-selection-panel-header agent-mode-panel-header"><h3 class="role-selection-panel-title">' + (wsT('chat.agentModePanelTitle') || '') + '</h3>' +
        '<button type="button" class="role-selection-panel-close" onclick="wsCloseAgentModePanel()"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path d="M18 6L6 18M6 6l12 12" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg></button>' +
        '</div>' +
        '<div class="agent-mode-options">' +
        '<button type="button" class="role-selection-item-main agent-mode-option ws-agent-mode-option" data-value="react" role="option" onclick="wsSelectAgentMode(\'react\')"><div class="role-selection-item-icon-main">\ud83e\udd16</div><div class="role-selection-item-content-main"><div class="role-selection-item-name-main">' + (wsT('chat.agentModeReactNative') || ' ReAct ') + '</div><div class="role-selection-item-description-main">' + (wsT('chat.agentModeReactNativeHint') || ' ReAct  MCP ') + '</div></div><div class="role-selection-checkmark-main agent-mode-check" data-agent-mode-check="react">\u2713</div></button>' +
        '<button type="button" class="role-selection-item-main agent-mode-option ws-agent-mode-option" data-value="eino_single" role="option" onclick="wsSelectAgentMode(\'eino_single\')"><div class="role-selection-item-icon-main">\u26a1</div><div class="role-selection-item-content-main"><div class="role-selection-item-name-main">' + (wsT('chat.agentModeEinoSingle') || 'Eino （ADK）') + '</div><div class="role-selection-item-description-main">' + (wsT('chat.agentModeEinoSingleHint') || 'Eino ChatModelAgent + Runner') + '</div></div><div class="role-selection-checkmark-main agent-mode-check" data-agent-mode-check="eino_single">\u2713</div></button>' +
        '<button type="button" class="role-selection-item-main agent-mode-option ws-agent-mode-option" data-value="deep" role="option" onclick="wsSelectAgentMode(\'deep\')"><div class="role-selection-item-icon-main">\ud83e\udde9</div><div class="role-selection-item-content-main"><div class="role-selection-item-name-main">' + (wsT('chat.agentModeDeep') || 'Deep（DeepAgent）') + '</div><div class="role-selection-item-description-main">' + (wsT('chat.agentModeDeepHint') || 'Eino DeepAgent，task ') + '</div></div><div class="role-selection-checkmark-main agent-mode-check" data-agent-mode-check="deep">\u2713</div></button>' +
        '<button type="button" class="role-selection-item-main agent-mode-option ws-agent-mode-option" data-value="plan_execute" role="option" onclick="wsSelectAgentMode(\'plan_execute\')"><div class="role-selection-item-icon-main">\ud83d\udccb</div><div class="role-selection-item-content-main"><div class="role-selection-item-name-main">' + (wsT('chat.agentModePlanExecuteLabel') || 'Plan-Execute') + '</div><div class="role-selection-item-description-main">' + (wsT('chat.agentModePlanExecuteHint') || ' →  → ') + '</div></div><div class="role-selection-checkmark-main agent-mode-check" data-agent-mode-check="plan_execute">\u2713</div></button>' +
        '<button type="button" class="role-selection-item-main agent-mode-option ws-agent-mode-option" data-value="supervisor" role="option" onclick="wsSelectAgentMode(\'supervisor\')"><div class="role-selection-item-icon-main">\ud83c\udfaf</div><div class="role-selection-item-content-main"><div class="role-selection-item-name-main">' + (wsT('chat.agentModeSupervisorLabel') || 'Supervisor') + '</div><div class="role-selection-item-description-main">' + (wsT('chat.agentModeSupervisorHint') || '，transfer ') + '</div></div><div class="role-selection-checkmark-main agent-mode-check" data-agent-mode-check="supervisor">\u2713</div></button>' +
        '</div></div></div>' +
        '<input type="hidden" id="ws-agent-mode-select" value="react" autocomplete="off" />' +
        '</div>' +
        '</div>' +
        '<div class="webshell-ai-input-row">' +
        '<textarea id="webshell-ai-input" class="webshell-ai-input form-control" rows="2" placeholder="' + (wsT('webshell.aiPlaceholder') || '：') + '"></textarea>' +
        '<button type="button" class="btn-primary" id="webshell-ai-send">' + (wsT('webshell.aiSend') || '') + '</button>' +
        '<button type="button" class="btn-danger webshell-ai-stop-btn" id="webshell-ai-stop" style="display:none;">' + wsTOr('webshell.aiStop', '') + '</button>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '</div>' +
        '<div id="webshell-pane-memo" class="webshell-pane webshell-pane-memo">' +
        '<div class="webshell-memo-layout">' +
        '<div class="webshell-memo-head"><span>' + (wsT('webshell.aiMemo') || '') + '</span><button type="button" class="btn-ghost btn-sm" id="webshell-ai-memo-clear">' + (wsT('webshell.aiMemoClear') || '') + '</button></div>' +
        '<textarea id="webshell-ai-memo-input" class="webshell-memo-input form-control" rows="18" placeholder="' + (wsT('webshell.aiMemoPlaceholder') || '、、...') + '"></textarea>' +
        '<div id="webshell-ai-memo-status" class="webshell-memo-status">' + (wsT('webshell.aiMemoSaved') || '') + '</div>' +
        '</div>' +
        '</div>' +
        '<div id="webshell-pane-db" class="webshell-pane webshell-pane-db">' +
        '<div class="webshell-db-profiles-bar"><div id="webshell-db-profiles" class="webshell-db-profiles"></div><div class="webshell-db-profile-actions"><button type="button" class="btn-ghost btn-sm" id="webshell-db-add-profile-btn">+ ' + (wsT('webshell.dbAddProfile') || '') + '</button></div></div>' +
        '<div class="webshell-db-layout">' +
        '<aside class="webshell-db-sidebar">' +
        '<div class="webshell-db-sidebar-head"><span>' + (wsT('webshell.dbSchema') || '') + '</span><button type="button" class="btn-ghost btn-sm" id="webshell-db-load-schema-btn">' + (wsT('webshell.dbLoadSchema') || '') + '</button></div>' +
        '<div id="webshell-db-schema-tree" class="webshell-db-schema-tree"><div class="webshell-empty">' + (wsT('webshell.dbNoSchema') || '，') + '</div></div>' +
        '<div class="webshell-db-sidebar-hint">' + (wsT('webshell.dbSelectTableHint') || ' SQL') + '</div>' +
        '</aside>' +
        '<section class="webshell-db-main">' +
        '<div class="webshell-db-sql-tools"><button type="button" class="btn-ghost btn-sm" id="webshell-db-template-btn">' + (wsT('webshell.dbTemplateSql') || ' SQL') + '</button><button type="button" class="btn-ghost btn-sm" id="webshell-db-clear-btn">' + (wsT('webshell.dbClearSql') || ' SQL') + '</button></div>' +
        '<textarea id="webshell-db-sql" class="webshell-db-sql form-control" rows="8" placeholder="' + (wsT('webshell.dbSqlPlaceholder') || ' SQL，：SELECT version();') + '"></textarea>' +
        '<div class="webshell-db-actions">' +
        '<button type="button" class="btn-ghost" id="webshell-db-test-btn">' + (wsT('webshell.dbTest') || '') + '</button>' +
        '<button type="button" class="btn-primary" id="webshell-db-run-btn">' + (wsT('webshell.dbRunSql') || ' SQL') + '</button>' +
        '</div>' +
        '<div class="webshell-db-output-wrap"><div class="webshell-db-output-title">' + (wsT('webshell.dbOutput') || '') + '</div><div id="webshell-db-result-table" class="webshell-db-result-table"></div><pre id="webshell-db-output" class="webshell-db-output"></pre><div class="webshell-db-hint">' + (wsT('webshell.dbCliHint') || '，（mysql/psql/sqlite3/sqlcmd）') + '</div></div>' +
        '<div id="webshell-db-profile-modal" class="modal">' +
        '<div class="modal-content webshell-db-profile-modal-content">' +
        '<div class="modal-header"><h2 id="webshell-db-profile-modal-title">' + (wsT('webshell.editConnectionTitle') || '') + '</h2><span class="modal-close" id="webshell-db-profile-modal-close">&times;</span></div>' +
        '<div class="modal-body">' +
        '<div class="webshell-db-toolbar">' +
        '<label><span>' + (wsT('webshell.dbProfileName') || '') + '</span><input id="webshell-db-profile-name" class="form-control" type="text" maxlength="30" /></label>' +
        '<label><span>' + (wsT('webshell.dbType') || '') + '</span><select id="webshell-db-type" class="form-control"><option value="mysql">MySQL</option><option value="pgsql">PostgreSQL</option><option value="sqlite">SQLite</option><option value="mssql">SQL Server</option></select></label>' +
        '<label class="webshell-db-common-field"><span>' + (wsT('webshell.dbHost') || '') + '</span><input id="webshell-db-host" class="form-control" type="text" value="127.0.0.1" /></label>' +
        '<label class="webshell-db-common-field"><span>' + (wsT('webshell.dbPort') || '') + '</span><input id="webshell-db-port" class="form-control" type="text" /></label>' +
        '<label class="webshell-db-common-field"><span>' + (wsT('webshell.dbUsername') || '') + '</span><input id="webshell-db-user" class="form-control" type="text" /></label>' +
        '<label class="webshell-db-common-field"><span>' + (wsT('webshell.dbPassword') || '') + '</span><input id="webshell-db-pass" class="form-control" type="password" /></label>' +
        '<label class="webshell-db-common-field"><span>' + (wsT('webshell.dbName') || '') + '</span><input id="webshell-db-name" class="form-control" type="text" /></label>' +
        '<label id="webshell-db-sqlite-row"><span>' + (wsT('webshell.dbSqlitePath') || 'SQLite ') + '</span><input id="webshell-db-sqlite-path" class="form-control" type="text" value="/tmp/test.db" /></label>' +
        '</div>' +
        '</div>' +
        '<div class="modal-footer"><button type="button" class="btn-secondary" id="webshell-db-profile-cancel-btn"></button><button type="button" class="btn-primary" id="webshell-db-profile-save-btn"></button></div>' +
        '</div>' +
        '</div>' +
        '</section>' +
        '</div>' +
        '</div>';

    // English note.
    workspace.querySelectorAll('.webshell-tab').forEach(btn => {
        btn.addEventListener('click', function () {
            const tab = btn.getAttribute('data-tab');
            workspace.querySelectorAll('.webshell-tab').forEach(b => b.classList.remove('active'));
            workspace.querySelectorAll('.webshell-pane').forEach(p => p.classList.remove('active'));
            btn.classList.add('active');
            const pane = document.getElementById('webshell-pane-' + tab);
            if (pane) pane.classList.add('active');
            if (tab === 'terminal' && webshellTerminalInstance && webshellTerminalFitAddon) {
                try { webshellTerminalFitAddon.fit(); } catch (e) {}
            }
            if (tab === 'ai') {
                try { wsRefreshSelectors(); } catch (e) {}
            }
        });
    });

    // English note.
    const pathInput = document.getElementById('webshell-file-path');
    document.getElementById('webshell-list-dir').addEventListener('click', function () {
        // English note.
        webshellFileListDir(webshellCurrentConn, pathInput ? pathInput.value.trim() || '.' : '.');
    });
    document.getElementById('webshell-parent-dir').addEventListener('click', function () {
        const p = (pathInput && pathInput.value.trim()) || '.';
        if (p === '.' || p === '/') {
            pathInput.value = '..';
        } else {
            pathInput.value = p.replace(/\/[^/]+$/, '') || '.';
        }
        webshellFileListDir(webshellCurrentConn, pathInput.value || '.');
    });

    // English note.
    var terminalCopyLogBtn = document.getElementById('webshell-terminal-copy-log');
    if (terminalCopyLogBtn) {
        terminalCopyLogBtn.addEventListener('click', function () {
            if (!webshellCurrentConn || !webshellCurrentConn.id) return;
            var activeId = getActiveWebshellTerminalSessionId(webshellCurrentConn.id);
            var log = getWebshellTerminalLog(getWebshellTerminalSessionKey(webshellCurrentConn.id, activeId)) || '';
            if (navigator && navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(log).then(function () {
                    terminalCopyLogBtn.title = wsT('webshell.terminalCopyOk') || '';
                    setTimeout(function () {
                        terminalCopyLogBtn.title = wsT('webshell.copyTerminalLog') || '';
                    }, 1200);
                }).catch(function () {
                    terminalCopyLogBtn.title = wsT('webshell.terminalCopyFail') || '';
                });
                return;
            }
            try {
                var ta = document.createElement('textarea');
                ta.value = log;
                document.body.appendChild(ta);
                ta.select();
                document.execCommand('copy');
                document.body.removeChild(ta);
            } catch (e) {}
        });
    }
    renderWebshellTerminalSessions(conn);
    // English note.
    workspace.querySelectorAll('.webshell-quick-cmd').forEach(function (btn) {
        btn.addEventListener('click', function () {
            var cmd = btn.getAttribute('data-cmd');
            if (cmd) runQuickCommand(cmd);
        });
    });
    // English note.
    var filterInput = document.getElementById('webshell-file-filter');
    document.getElementById('webshell-file-refresh').addEventListener('click', function () {
        webshellFileListDir(webshellCurrentConn, pathInput ? pathInput.value.trim() || '.' : '.');
    });
    if (filterInput) filterInput.addEventListener('input', function () {
        webshellFileListApplyFilter();
    });
    document.getElementById('webshell-mkdir-btn').addEventListener('click', function () { webshellFileMkdir(webshellCurrentConn, pathInput); });
    document.getElementById('webshell-newfile-btn').addEventListener('click', function () { webshellFileNewFile(webshellCurrentConn, pathInput); });
    document.getElementById('webshell-upload-btn').addEventListener('click', function () { webshellFileUpload(webshellCurrentConn, pathInput); });
    document.getElementById('webshell-batch-delete-btn').addEventListener('click', function () { webshellBatchDelete(webshellCurrentConn, pathInput); });
    document.getElementById('webshell-batch-download-btn').addEventListener('click', function () { webshellBatchDownload(webshellCurrentConn, pathInput); });

    // English note.
    var aiInput = document.getElementById('webshell-ai-input');
    var aiSendBtn = document.getElementById('webshell-ai-send');
    var aiMessages = document.getElementById('webshell-ai-messages');
    var aiNewConvBtn = document.getElementById('webshell-ai-new-conv');
    var aiConvListEl = document.getElementById('webshell-ai-conv-list');

    // English note.
    wsLoadRoles();
    wsInitAgentMode();
    var aiMemoInput = document.getElementById('webshell-ai-memo-input');
    var aiMemoStatus = document.getElementById('webshell-ai-memo-status');
    var aiMemoClearBtn = document.getElementById('webshell-ai-memo-clear');
    var aiMemoSaveTimer = null;

    function setWebshellAiMemoStatus(text, isError) {
        if (!aiMemoStatus) return;
        aiMemoStatus.textContent = text || '';
        aiMemoStatus.classList.toggle('error', !!isError);
    }

    function flushWebshellAiMemo() {
        if (!aiMemoInput) return;
        saveWebshellAiMemo(conn, aiMemoInput.value || '');
        setWebshellAiMemoStatus(wsT('webshell.aiMemoSaved') || '', false);
    }

    if (aiMemoInput) {
        aiMemoInput.value = getWebshellAiMemo(conn);
        setWebshellAiMemoStatus(wsT('webshell.aiMemoSaved') || '', false);
        aiMemoInput.addEventListener('input', function () {
            setWebshellAiMemoStatus(wsT('webshell.aiMemoSaving') || '...', false);
            if (aiMemoSaveTimer) clearTimeout(aiMemoSaveTimer);
            aiMemoSaveTimer = setTimeout(function () {
                aiMemoSaveTimer = null;
                flushWebshellAiMemo();
            }, 500);
        });
        aiMemoInput.addEventListener('blur', function () {
            if (aiMemoSaveTimer) {
                clearTimeout(aiMemoSaveTimer);
                aiMemoSaveTimer = null;
            }
            flushWebshellAiMemo();
        });
    }

    if (aiMemoClearBtn && aiMemoInput) {
        aiMemoClearBtn.addEventListener('click', function () {
            aiMemoInput.value = '';
            flushWebshellAiMemo();
            aiMemoInput.focus();
        });
    }

    if (aiNewConvBtn) {
        aiNewConvBtn.addEventListener('click', function () {
            delete webshellAiConvMap[conn.id];
            if (aiMessages) {
                aiMessages.innerHTML = '';
                var readyMsg = wsT('webshell.aiSystemReadyMessage') || '。，。';
                var div = document.createElement('div');
                div.className = 'webshell-ai-msg assistant';
                div.textContent = readyMsg;
                aiMessages.appendChild(div);
            }
            if (aiConvListEl) aiConvListEl.querySelectorAll('.webshell-ai-conv-item').forEach(function (el) { el.classList.remove('active'); });
        });
    }
    if (aiSendBtn && aiInput && aiMessages) {
        var aiStopBtn = document.getElementById('webshell-ai-stop');
        aiSendBtn.addEventListener('click', function () { runWebshellAiSend(conn, aiInput, aiSendBtn, aiMessages); });
        if (aiStopBtn) {
            aiStopBtn.addEventListener('click', function () { wsStopAiStream(conn); });
        }
        aiInput.addEventListener('keydown', function (e) {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                runWebshellAiSend(conn, aiInput, aiSendBtn, aiMessages);
            }
        });
        fetchAndRenderWebshellAiConvList(conn, aiConvListEl).then(function () {
            loadWebshellAiHistory(conn, aiMessages).then(function () {
                if (webshellAiConvMap[conn.id] && aiConvListEl) {
                    aiConvListEl.querySelectorAll('.webshell-ai-conv-item').forEach(function (el) {
                        el.classList.toggle('active', el.dataset.convId === webshellAiConvMap[conn.id]);
                    });
                }
            });
        });
    }

    // English note.
    var dbTypeEl = document.getElementById('webshell-db-type');
    var dbRunBtn = document.getElementById('webshell-db-run-btn');
    var dbTestBtn = document.getElementById('webshell-db-test-btn');
    var dbSqlEl = document.getElementById('webshell-db-sql');
    var dbLoadSchemaBtn = document.getElementById('webshell-db-load-schema-btn');
    var dbTemplateBtn = document.getElementById('webshell-db-template-btn');
    var dbClearBtn = document.getElementById('webshell-db-clear-btn');
    var dbSchemaTreeEl = document.getElementById('webshell-db-schema-tree');
    var dbProfilesEl = document.getElementById('webshell-db-profiles');
    var dbAddProfileBtn = document.getElementById('webshell-db-add-profile-btn');
    var dbProfileModalEl = document.getElementById('webshell-db-profile-modal');
    var dbProfileModalTitleEl = document.getElementById('webshell-db-profile-modal-title');
    var dbProfileModalCloseBtn = document.getElementById('webshell-db-profile-modal-close');
    var dbProfileModalCancelBtn = document.getElementById('webshell-db-profile-cancel-btn');
    var dbProfileModalSaveBtn = document.getElementById('webshell-db-profile-save-btn');
    var dbProfileNameEl = document.getElementById('webshell-db-profile-name');
    var dbHostEl = document.getElementById('webshell-db-host');
    var dbPortEl = document.getElementById('webshell-db-port');
    var dbUserEl = document.getElementById('webshell-db-user');
    var dbPassEl = document.getElementById('webshell-db-pass');
    var dbNameEl = document.getElementById('webshell-db-name');
    var dbSqliteEl = document.getElementById('webshell-db-sqlite-path');
    var dbColumnsLoading = {};
    var dbColumnsBatchLoading = {};
    var dbColumnsBatchLoaded = {};

    function resetDbColumnLoadCache() {
        dbColumnsLoading = {};
        dbColumnsBatchLoading = {};
        dbColumnsBatchLoaded = {};
    }

    function setDbActionButtonsDisabled(disabled) {
        if (dbRunBtn) dbRunBtn.disabled = disabled;
        if (dbTestBtn) dbTestBtn.disabled = disabled;
        if (dbLoadSchemaBtn) dbLoadSchemaBtn.disabled = disabled;
        if (dbAddProfileBtn) dbAddProfileBtn.disabled = disabled;
    }

    function setDbProfileModalVisible(visible, mode) {
        if (!dbProfileModalEl) return;
        dbProfileModalEl.style.display = visible ? 'block' : 'none';
        if (dbProfileModalTitleEl) {
            if (mode === 'add') dbProfileModalTitleEl.textContent = wsT('webshell.dbAddProfile') || '';
            else dbProfileModalTitleEl.textContent = wsT('webshell.editConnectionTitle') || '';
        }
    }

    function applyActiveDbProfileToForm() {
        var dbCfg = getWebshellDbConfig(conn);
        if (!dbCfg) return;
        if (dbProfileNameEl) dbProfileNameEl.value = dbCfg.name || 'DB-1';
        if (dbTypeEl) dbTypeEl.value = dbCfg.type || 'mysql';
        if (dbHostEl) dbHostEl.value = dbCfg.host || '127.0.0.1';
        if (dbPortEl) dbPortEl.value = dbCfg.port || '';
        if (dbUserEl) dbUserEl.value = dbCfg.username || '';
        if (dbPassEl) dbPassEl.value = dbCfg.password || '';
        if (dbNameEl) dbNameEl.value = dbCfg.database || dbCfg.selectedDatabase || '';
        if (dbSqliteEl) dbSqliteEl.value = dbCfg.sqlitePath || '/tmp/test.db';
        if (dbSqlEl) dbSqlEl.value = dbCfg.sql || 'SELECT 1;';
        webshellDbUpdateFieldVisibility();
        webshellDbSetOutput(dbCfg.output || '', !!dbCfg.outputIsError);
        webshellDbRenderTable(dbCfg.output || '');
    }

    function renderDbProfileTabs() {
        if (!dbProfilesEl) return;
        var state = getWebshellDbState(conn);
        var html = '';
        state.profiles.forEach(function (p) {
            var active = p.id === state.activeProfileId;
            html += '<div class="webshell-db-profile-tab' + (active ? ' active' : '') + '" data-id="' + escapeHtml(p.id) + '">' +
                '<button type="button" class="webshell-db-profile-main" data-action="switch" data-id="' + escapeHtml(p.id) + '">' + escapeHtml(p.name || 'DB') + '</button>' +
                '<button type="button" class="webshell-db-profile-menu" data-action="edit" data-id="' + escapeHtml(p.id) + '" title="' + escapeHtml(wsT('webshell.editConnection') || '') + '">⚙</button>' +
                '<button type="button" class="webshell-db-profile-menu" data-action="delete" data-id="' + escapeHtml(p.id) + '" title="' + escapeHtml(wsT('webshell.dbDeleteProfile') || '') + '">×</button>' +
                '</div>';
        });
        dbProfilesEl.innerHTML = html;
        dbProfilesEl.querySelectorAll('[data-action]').forEach(function (btn) {
            btn.addEventListener('click', function () {
                var action = btn.getAttribute('data-action');
                var id = btn.getAttribute('data-id') || '';
                if (!id) return;
                var state = getWebshellDbState(conn);
                var idx = state.profiles.findIndex(function (p) { return p.id === id; });
                if (idx < 0) return;
                if (action === 'switch') {
                    state.activeProfileId = id;
                    saveWebshellDbState(conn, state);
                    applyActiveDbProfileToForm();
                    renderDbProfileTabs();
                    resetDbColumnLoadCache();
                    renderDbSchemaTree();
                    return;
                }
                if (action === 'edit') {
                    state.activeProfileId = id;
                    saveWebshellDbState(conn, state);
                    applyActiveDbProfileToForm();
                    renderDbProfileTabs();
                    setDbProfileModalVisible(true, 'edit');
                    return;
                }
                if (action === 'delete') {
                    if (state.profiles.length <= 1) return;
                    if (!confirm(wsT('webshell.dbDeleteProfileConfirm') || '？')) return;
                    state.profiles.splice(idx, 1);
                    if (!state.profiles.some(function (p) { return p.id === state.activeProfileId; })) {
                        state.activeProfileId = state.profiles[0].id;
                    }
                    saveWebshellDbState(conn, state);
                    applyActiveDbProfileToForm();
                    renderDbProfileTabs();
                    resetDbColumnLoadCache();
                    renderDbSchemaTree();
                }
            });
        });
    }

    function renderDbSchemaTree() {
        if (!dbSchemaTreeEl) return;
        var cfg = getWebshellDbConfig(conn);
        var schema = normalizeWebshellDbSchema((cfg && cfg.schema && typeof cfg.schema === 'object') ? cfg.schema : {});
        var dbs = Object.keys(schema).sort(function (a, b) { return a.localeCompare(b); });
        var openTableKeys = {};
        dbSchemaTreeEl.querySelectorAll('.webshell-db-table-node[open]').forEach(function (node) {
            var openDb = node.getAttribute('data-db') || '';
            var openTable = node.getAttribute('data-table') || '';
            if (!openDb || !openTable) return;
            openTableKeys[openDb + '::' + openTable] = true;
        });
        if (!dbs.length) {
            dbSchemaTreeEl.innerHTML = '<div class="webshell-empty">' + escapeHtml(wsT('webshell.dbNoSchema') || '，') + '</div>';
            return;
        }
        var selectedDb = (cfg.selectedDatabase || '').trim();
        var html = '';
        dbs.forEach(function (dbName) {
            var tables = (schema[dbName] && schema[dbName].tables) ? schema[dbName].tables : {};
            var tableNames = Object.keys(tables).sort(function (a, b) { return a.localeCompare(b); });
            var isActive = selectedDb && selectedDb === dbName;
            html += '<details class="webshell-db-group"' + (isActive ? ' open' : '') + '>';
            html += '<summary class="webshell-db-group-title" data-db="' + escapeHtml(dbName) + '" title="' + escapeHtml(dbName) + '"><span class="webshell-db-icon">🗄</span><span class="webshell-db-label">' + escapeHtml(dbName) + '</span><span class="webshell-db-count">' + tableNames.length + '</span></summary>';
            html += '<div class="webshell-db-group-items">';
            tableNames.forEach(function (tableName) {
                var columns = Array.isArray(tables[tableName]) ? tables[tableName] : [];
                var columnCountText = columns.length > 0 ? String(columns.length) : '-';
                var tableKey = dbName + '::' + tableName;
                var tableOpen = !!openTableKeys[tableKey];
                html += '<details class="webshell-db-table-node" data-db="' + escapeHtml(dbName) + '" data-table="' + escapeHtml(tableName) + '" data-columns-loaded="' + (columns.length ? '1' : '0') + '"' + (tableOpen ? ' open' : '') + '>';
                html += '<summary class="webshell-db-table-item" data-db="' + escapeHtml(dbName) + '" data-table="' + escapeHtml(tableName) + '" title="' + escapeHtml(tableName) + '"><span class="webshell-db-icon">📄</span><span class="webshell-db-label">' + escapeHtml(tableName) + '</span><span class="webshell-db-count">' + escapeHtml(columnCountText) + '</span></summary>';
                if (columns.length) {
                    html += '<div class="webshell-db-column-list">';
                    columns.forEach(function (columnName) {
                        html += '<button type="button" class="webshell-db-column-item" data-db="' + escapeHtml(dbName) + '" data-table="' + escapeHtml(tableName) + '" data-column="' + escapeHtml(columnName) + '" title="' + escapeHtml(columnName) + '"><span class="webshell-db-icon">🧱</span><span class="webshell-db-label">' + escapeHtml(columnName) + '</span></button>';
                    });
                    html += '</div>';
                } else {
                    html += '<div class="webshell-db-column-empty">' + escapeHtml(wsT('webshell.dbNoColumns') || '') + '</div>';
                }
                html += '</details>';
            });
            html += '</div></details>';
        });
        dbSchemaTreeEl.innerHTML = html;

        dbSchemaTreeEl.querySelectorAll('.webshell-db-group-title').forEach(function (el) {
            el.addEventListener('click', function () {
                var cfg = webshellDbCollectConfig(conn);
                cfg.selectedDatabase = el.getAttribute('data-db') || '';
                saveWebshellDbConfig(conn, cfg);
                if (dbNameEl && cfg.type !== 'sqlite') dbNameEl.value = cfg.selectedDatabase;
                ensureDbDatabaseColumns(cfg.selectedDatabase);
            });
        });
        dbSchemaTreeEl.querySelectorAll('.webshell-db-table-item').forEach(function (el) {
            el.addEventListener('click', function () {
                var table = el.getAttribute('data-table') || '';
                var dbName = el.getAttribute('data-db') || '';
                if (!table) return;
                var cfg = webshellDbCollectConfig(conn);
                cfg.selectedDatabase = dbName;
                if (cfg.type !== 'sqlite') cfg.database = dbName;
                saveWebshellDbConfig(conn, cfg);
                if (dbNameEl && cfg.type !== 'sqlite') dbNameEl.value = dbName;
                var tableRef = cfg.type === 'sqlite'
                    ? webshellDbQuoteIdentifier(cfg.type, table)
                    : webshellDbQuoteIdentifier(cfg.type, dbName) + '.' + webshellDbQuoteIdentifier(cfg.type, table);
                if (dbSqlEl) {
                    dbSqlEl.value = 'SELECT * FROM ' + tableRef + ' ORDER BY 1 DESC LIMIT 20;';
                    webshellDbCollectConfig(conn);
                }
                ensureDbTableColumns(dbName, table);
            });
        });
        dbSchemaTreeEl.querySelectorAll('.webshell-db-table-node').forEach(function (node) {
            node.addEventListener('toggle', function () {
                if (!node.open) return;
                var dbName = node.getAttribute('data-db') || '';
                var table = node.getAttribute('data-table') || '';
                if (!dbName || !table) return;
                ensureDbTableColumns(dbName, table);
            });
        });
        dbSchemaTreeEl.querySelectorAll('.webshell-db-column-item').forEach(function (el) {
            el.addEventListener('click', function (evt) {
                evt.preventDefault();
                evt.stopPropagation();
                var table = el.getAttribute('data-table') || '';
                var column = el.getAttribute('data-column') || '';
                var dbName = el.getAttribute('data-db') || '';
                if (!table || !column) return;
                var cfg = webshellDbCollectConfig(conn);
                cfg.selectedDatabase = dbName;
                if (cfg.type !== 'sqlite') cfg.database = dbName;
                saveWebshellDbConfig(conn, cfg);
                if (dbNameEl && cfg.type !== 'sqlite') dbNameEl.value = dbName;
                var tableRef = cfg.type === 'sqlite'
                    ? webshellDbQuoteIdentifier(cfg.type, table)
                    : webshellDbQuoteIdentifier(cfg.type, dbName) + '.' + webshellDbQuoteIdentifier(cfg.type, table);
                if (dbSqlEl) {
                    dbSqlEl.value = 'SELECT ' + webshellDbQuoteIdentifier(cfg.type, column) + ' FROM ' + tableRef + ' LIMIT 20;';
                    webshellDbCollectConfig(conn);
                }
            });
        });
        var autoDb = selectedDb || dbs[0] || '';
        if (autoDb) ensureDbDatabaseColumns(autoDb);
    }

    function ensureDbTableColumns(dbName, tableName) {
        if (!dbName || !tableName || webshellRunning) return;
        var loadKey = (conn && conn.id ? conn.id : 'local') + '::' + dbName + '::' + tableName;
        if (dbColumnsLoading[loadKey]) return;
        webshellDbCollectConfig(conn);
        var cfg = getWebshellDbConfig(conn);
        var schema = normalizeWebshellDbSchema((cfg && cfg.schema && typeof cfg.schema === 'object') ? cfg.schema : {});
        if (!schema[dbName]) return;
        if (!schema[dbName].tables[tableName]) return;
        if (Array.isArray(schema[dbName].tables[tableName]) && schema[dbName].tables[tableName].length > 0) return;

        var built = buildWebshellDbColumnsCommand(cfg, dbName, tableName);
        if (!built.command) return;
        dbColumnsLoading[loadKey] = true;
        webshellRunning = true;
        setDbActionButtonsDisabled(true);
        execWebshellCommand(conn, built.command).then(function (out) {
            var parsed = parseWebshellDbExecOutput(out);
            var success = parsed.rc === 0 || (parsed.rc == null && parsed.output && !/error|failed|denied|unknown|not found|access/i.test(parsed.output));
            if (!success) return;
            var columns = parseWebshellDbColumns(parsed.output);
            if (!columns.length) return;
            var nextCfg = getWebshellDbConfig(conn);
            var nextSchema = normalizeWebshellDbSchema((nextCfg && nextCfg.schema && typeof nextCfg.schema === 'object') ? nextCfg.schema : {});
            if (!nextSchema[dbName]) nextSchema[dbName] = { tables: {} };
            if (!nextSchema[dbName].tables[tableName]) nextSchema[dbName].tables[tableName] = [];
            nextSchema[dbName].tables[tableName] = columns;
            nextCfg.schema = nextSchema;
            saveWebshellDbConfig(conn, nextCfg);
            renderDbSchemaTree();
        }).catch(function () {
            // ignore single-table column load errors to avoid interrupting main flow
        }).finally(function () {
            delete dbColumnsLoading[loadKey];
            webshellRunning = false;
            setDbActionButtonsDisabled(false);
        });
    }

    function ensureDbDatabaseColumns(dbName) {
        if (!dbName || webshellRunning) return;
        webshellDbCollectConfig(conn);
        var cfg = getWebshellDbConfig(conn);
        var schema = normalizeWebshellDbSchema((cfg && cfg.schema && typeof cfg.schema === 'object') ? cfg.schema : {});
        if (!schema[dbName] || !schema[dbName].tables) return;
        var hasUnknown = Object.keys(schema[dbName].tables).some(function (tableName) {
            var cols = schema[dbName].tables[tableName];
            return !Array.isArray(cols) || cols.length === 0;
        });
        if (!hasUnknown) return;

        var batchKey = (conn && conn.id ? conn.id : 'local') + '::' + (cfg.type || 'mysql') + '::' + dbName;
        if (dbColumnsBatchLoading[batchKey] || dbColumnsBatchLoaded[batchKey]) return;
        var built = buildWebshellDbColumnsByDatabaseCommand(cfg, dbName);
        if (!built.command) return;

        dbColumnsBatchLoading[batchKey] = true;
        webshellRunning = true;
        setDbActionButtonsDisabled(true);
        execWebshellCommand(conn, built.command).then(function (out) {
            var parsed = parseWebshellDbExecOutput(out);
            var success = parsed.rc === 0 || (parsed.rc == null && parsed.output && !/error|failed|denied|unknown|not found|access/i.test(parsed.output));
            if (!success) return;
            var tableColumns = parseWebshellDbTableColumns(parsed.output);
            if (!Object.keys(tableColumns).length) return;

            var nextCfg = getWebshellDbConfig(conn);
            var nextSchema = normalizeWebshellDbSchema((nextCfg && nextCfg.schema && typeof nextCfg.schema === 'object') ? nextCfg.schema : {});
            if (!nextSchema[dbName]) nextSchema[dbName] = { tables: {} };
            Object.keys(tableColumns).forEach(function (tableName) {
                nextSchema[dbName].tables[tableName] = tableColumns[tableName];
            });
            nextCfg.schema = nextSchema;
            saveWebshellDbConfig(conn, nextCfg);
            renderDbSchemaTree();
        }).catch(function () {
            // ignore batch column load errors to avoid interrupting main flow
        }).finally(function () {
            delete dbColumnsBatchLoading[batchKey];
            dbColumnsBatchLoaded[batchKey] = true;
            webshellRunning = false;
            setDbActionButtonsDisabled(false);
        });
    }

    function loadDbSchema() {
        if (!conn || !conn.id) {
            webshellDbSetOutput(wsT('webshell.dbNoConn') || ' WebShell ', true);
            return;
        }
        if (webshellRunning) {
            webshellDbSetOutput(wsT('webshell.dbRunning') || '，', true);
            return;
        }
        var cfg = webshellDbCollectConfig(conn);
        resetDbColumnLoadCache();
        var built = buildWebshellDbSchemaCommand(cfg);
        if (!built.command) {
            webshellDbSetOutput(built.error || (wsT('webshell.dbSchemaFailed') || ''), true);
            return;
        }
        webshellDbSetOutput(wsT('webshell.running') || '…', false);
        webshellRunning = true;
        setDbActionButtonsDisabled(true);
        execWebshellCommand(conn, built.command).then(function (out) {
            var parsed = parseWebshellDbExecOutput(out);
            var success = parsed.rc === 0 || (parsed.rc == null && parsed.output && !/error|failed|denied|unknown|not found|access/i.test(parsed.output));
            if (!success) {
                webshellDbSetOutput((wsT('webshell.dbSchemaFailed') || '') + ':\n' + (parsed.output || ''), true);
                return;
            }
            cfg.schema = parseWebshellDbSchema(parsed.output);
            cfg.output = wsT('webshell.dbSchemaLoaded') || '';
            cfg.outputIsError = false;
            saveWebshellDbConfig(conn, cfg);
            renderDbSchemaTree();
            webshellDbSetOutput(wsT('webshell.dbSchemaLoaded') || '', false);
        }).catch(function (err) {
            webshellDbSetOutput((wsT('webshell.dbSchemaFailed') || '') + ': ' + (err && err.message ? err.message : String(err)), true);
        }).finally(function () {
            webshellRunning = false;
            setDbActionButtonsDisabled(false);
        });
    }

    function runDbQuery(isTestOnly) {
        if (!conn || !conn.id) {
            webshellDbSetOutput(wsT('webshell.dbNoConn') || ' WebShell ', true);
            return;
        }
        if (webshellRunning) {
            webshellDbSetOutput(wsT('webshell.dbRunning') || '，', true);
            return;
        }
        var cfg = webshellDbCollectConfig(conn);
        var built = buildWebshellDbCommand(cfg, !!isTestOnly);
        if (!built.command) {
            webshellDbSetOutput(built.error || (wsT('webshell.dbExecFailed') || ''), true);
            return;
        }
        webshellDbSetOutput(wsT('webshell.running') || '…', false);
        webshellRunning = true;
        setDbActionButtonsDisabled(true);
        execWebshellCommand(conn, built.command).then(function (out) {
            var parsed = parseWebshellDbExecOutput(out);
            var code = parsed.rc;
            var content = parsed.output || '';
            var success = (code === 0) || (code == null && content && !/error|failed|denied|unknown|not found|access/i.test(content));
            if (isTestOnly) {
                if (success) {
                    cfg.output = '';
                    cfg.outputIsError = false;
                    saveWebshellDbConfig(conn, cfg);
                    webshellDbSetOutput(cfg.output, false);
                } else {
                    cfg.output = '' + (content ? (':\n' + content) : '');
                    cfg.outputIsError = true;
                    saveWebshellDbConfig(conn, cfg);
                    webshellDbSetOutput(cfg.output, true);
                }
                return;
            }
            if (!success) {
                cfg.output = (wsT('webshell.dbExecFailed') || '') + (content ? (':\n' + content) : '');
                cfg.outputIsError = true;
                saveWebshellDbConfig(conn, cfg);
                webshellDbSetOutput(cfg.output, true);
                return;
            }
            var hasTable = webshellDbRenderTable(content);
            if (hasTable) {
                cfg.output = wsT('webshell.dbExecSuccess') || 'SQL ';
                cfg.outputIsError = false;
                saveWebshellDbConfig(conn, cfg);
                webshellDbSetOutput(cfg.output, false);
            } else {
                cfg.output = content || (wsT('webshell.dbNoOutput') || '（）');
                cfg.outputIsError = false;
                saveWebshellDbConfig(conn, cfg);
                webshellDbSetOutput(cfg.output, false);
            }
        }).catch(function (err) {
            cfg.output = (wsT('webshell.dbExecFailed') || '') + ': ' + (err && err.message ? err.message : String(err));
            cfg.outputIsError = true;
            saveWebshellDbConfig(conn, cfg);
            webshellDbSetOutput(cfg.output, true);
        }).finally(function () {
            webshellRunning = false;
            setDbActionButtonsDisabled(false);
        });
    }

    if (dbTypeEl) dbTypeEl.addEventListener('change', function () {
        webshellDbUpdateFieldVisibility();
        var cfg = webshellDbCollectConfig(conn);
        cfg.selectedDatabase = '';
        cfg.schema = {};
        saveWebshellDbConfig(conn, cfg);
        resetDbColumnLoadCache();
        renderDbSchemaTree();
    });
    ['webshell-db-profile-name', 'webshell-db-host', 'webshell-db-port', 'webshell-db-user', 'webshell-db-pass', 'webshell-db-name', 'webshell-db-sqlite-path'].forEach(function (id) {
        var el = document.getElementById(id);
        if (el) el.addEventListener('change', function () {
            webshellDbCollectConfig(conn);
            resetDbColumnLoadCache();
            renderDbProfileTabs();
        });
    });
    if (dbSqlEl) dbSqlEl.addEventListener('change', function () { webshellDbCollectConfig(conn); });
    if (dbRunBtn) dbRunBtn.addEventListener('click', function () { runDbQuery(false); });
    if (dbTestBtn) dbTestBtn.addEventListener('click', function () { runDbQuery(true); });
    if (dbLoadSchemaBtn) dbLoadSchemaBtn.addEventListener('click', function () { loadDbSchema(); });
    if (dbTemplateBtn) dbTemplateBtn.addEventListener('click', function () {
        if (!dbSqlEl) return;
        var cfg = webshellDbCollectConfig(conn);
        if (cfg.type === 'mysql') dbSqlEl.value = 'SHOW DATABASES;\nSELECT DATABASE() AS current_db;';
        else if (cfg.type === 'pgsql') dbSqlEl.value = 'SELECT current_database();\nSELECT schema_name FROM information_schema.schemata ORDER BY schema_name;';
        else if (cfg.type === 'sqlite') dbSqlEl.value = "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name;";
        else dbSqlEl.value = "SELECT name FROM sys.databases ORDER BY name;\nSELECT DB_NAME() AS current_db;";
        webshellDbCollectConfig(conn);
    });
    if (dbClearBtn) dbClearBtn.addEventListener('click', function () {
        if (dbSqlEl) dbSqlEl.value = '';
        webshellDbCollectConfig(conn);
    });
    if (dbProfileModalCloseBtn) dbProfileModalCloseBtn.addEventListener('click', function () {
        setDbProfileModalVisible(false);
    });
    if (dbProfileModalCancelBtn) dbProfileModalCancelBtn.addEventListener('click', function () {
        applyActiveDbProfileToForm();
        setDbProfileModalVisible(false);
    });
    if (dbProfileModalSaveBtn) dbProfileModalSaveBtn.addEventListener('click', function () {
        webshellDbCollectConfig(conn);
        renderDbProfileTabs();
        resetDbColumnLoadCache();
        setDbProfileModalVisible(false);
    });
    if (dbProfileModalEl) dbProfileModalEl.addEventListener('click', function (evt) {
        if (evt.target === dbProfileModalEl) {
            applyActiveDbProfileToForm();
            setDbProfileModalVisible(false);
        }
    });
    if (dbAddProfileBtn) dbAddProfileBtn.addEventListener('click', function () {
        var state = getWebshellDbState(conn);
        var name = 'DB-' + (state.profiles.length + 1);
        var p = newWebshellDbProfile(name);
        state.profiles.push(p);
        state.activeProfileId = p.id;
        saveWebshellDbState(conn, state);
        applyActiveDbProfileToForm();
        renderDbProfileTabs();
        renderDbSchemaTree();
        setDbProfileModalVisible(true, 'add');
    });
    renderDbProfileTabs();
    applyActiveDbProfileToForm();
    renderDbSchemaTree();
    setDbProfileModalVisible(false);

    initWebshellTerminal(conn);
}

// English note.
function loadWebshellAiHistory(conn, messagesContainer) {
    if (!conn || !conn.id || !messagesContainer) return Promise.resolve();
    if (typeof apiFetch !== 'function') return Promise.resolve();
    return apiFetch('/api/webshell/connections/' + encodeURIComponent(conn.id) + '/ai-history', { method: 'GET' })
        .then(function (r) { return r.json(); })
        .then(function (data) {
            if (data.conversationId) webshellAiConvMap[conn.id] = data.conversationId;
            var list = Array.isArray(data.messages) ? data.messages : [];
            list.forEach(function (msg) {
                var role = (msg.role || '').toLowerCase();
                var content = (msg.content || '').trim();
                if (!content && role !== 'assistant') return;
                var div = document.createElement('div');
                div.className = 'webshell-ai-msg ' + (role === 'user' ? 'user' : 'assistant');
                if (role === 'user') {
                    div.textContent = content;
                } else {
                    if (isLikelyWebshellAiErrorMessage(content, msg)) {
                        renderWebshellAiErrorMessage(div, content);
                    } else if (typeof formatMarkdown === 'function') {
                        div.innerHTML = formatMarkdown(content);
                    } else {
                        div.textContent = content;
                    }
                }
                messagesContainer.appendChild(div);
                if (role === 'assistant' && msg.processDetails && msg.processDetails.length > 0) {
                    var block = renderWebshellProcessDetailsBlock(msg.processDetails, true);
                    if (block) messagesContainer.appendChild(block);
                }
            });
            if (list.length === 0) {
                var readyMsg = wsT('webshell.aiSystemReadyMessage') || '。，。';
                var readyDiv = document.createElement('div');
                readyDiv.className = 'webshell-ai-msg assistant';
                readyDiv.textContent = readyMsg;
                messagesContainer.appendChild(readyDiv);
            }
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
        })
        .catch(function (e) {
            console.warn(' WebShell AI ', conn.id, e);
        });
}

function runWebshellAiSend(conn, inputEl, sendBtn, messagesContainer) {
    if (!conn || !conn.id) return;
    var message = (inputEl && inputEl.value || '').trim();
    if (!message) return;
    if (webshellAiSending) return;
    if (typeof apiFetch !== 'function') {
        if (messagesContainer) {
            var errDiv = document.createElement('div');
            errDiv.className = 'webshell-ai-msg assistant';
            errDiv.textContent = '： apiFetch ';
            messagesContainer.appendChild(errDiv);
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
        }
        return;
    }

    webshellAiAbortController = new AbortController();
    wsSetAiSendingState(true);

    var userDiv = document.createElement('div');
    userDiv.className = 'webshell-ai-msg user';
    userDiv.textContent = message;
    messagesContainer.appendChild(userDiv);

    var timelineContainer = document.createElement('div');
    timelineContainer.className = 'webshell-ai-timeline';
    timelineContainer.setAttribute('aria-live', 'polite');

    var assistantDiv = document.createElement('div');
    assistantDiv.className = 'webshell-ai-msg assistant';
    assistantDiv.textContent = '…';
    messagesContainer.appendChild(timelineContainer);
    messagesContainer.appendChild(assistantDiv);
    messagesContainer.scrollTop = messagesContainer.scrollHeight;

    function appendTimelineItem(type, title, message, data) {
        var item = document.createElement('div');
        item.className = 'webshell-ai-timeline-item webshell-ai-timeline-' + type;

        var html = '<span class="webshell-ai-timeline-title">' + escapeHtml(title || message || '') + '</span>';

        // English note.
        if (type === 'tool_call' && data) {
            try {
                var args = data.argumentsObj;
                if (args == null && data.arguments != null && String(data.arguments).trim() !== '') {
                    try {
                        args = JSON.parse(String(data.arguments));
                    } catch (e1) {
                        args = { _raw: String(data.arguments) };
                    }
                }
                if (args && typeof args === 'object') {
                    var paramsLabel = (typeof window.t === 'function') ? window.t('timeline.params') : ':';
                    html += '<div class="webshell-ai-timeline-msg"><div class="tool-arg-section"><strong>' +
                        escapeHtml(paramsLabel) +
                        '</strong><pre class="tool-args">' +
                        escapeHtml(JSON.stringify(args, null, 2)) +
                        '</pre></div></div>';
                }
            } catch (e) {
                // English note.
            }
        } else if (type === 'eino_agent_reply' && message) {
            html += '<div class="webshell-ai-timeline-msg"><pre style="white-space:pre-wrap;">' + escapeHtml(message) + '</pre></div>';
        } else if (type === 'tool_result' && data) {
            // English note.
            var isError = data.isError || data.success === false;
            var noResultText = (typeof window.t === 'function') ? window.t('timeline.noResult') : '';
            var result = data.result != null ? data.result : (data.error != null ? data.error : noResultText);
            var resultStr = (typeof result === 'string') ? result : JSON.stringify(result);
            var execResultLabel = (typeof window.t === 'function') ? window.t('timeline.executionResult') : ':';
            var execIdLabel = (typeof window.t === 'function') ? window.t('timeline.executionId') : 'ID:';
            html += '<div class="webshell-ai-timeline-msg"><div class="tool-result-section ' +
                (isError ? 'error' : 'success') +
                '"><strong>' + escapeHtml(execResultLabel) + '</strong><pre class="tool-result">' +
                escapeHtml(resultStr) +
                '</pre>' +
                (data.executionId ? '<div class="tool-execution-id"><span>' +
                    escapeHtml(execIdLabel) +
                    '</span> <code>' +
                    escapeHtml(String(data.executionId)) +
                    '</code></div>' : '') +
                '</div></div>';
        } else if (message && message !== title) {
            html += '<div class="webshell-ai-timeline-msg">' + escapeHtml(message) + '</div>';
        }

        item.innerHTML = html;
        timelineContainer.appendChild(item);
        timelineContainer.classList.add('has-items');
        messagesContainer.scrollTop = messagesContainer.scrollHeight;
        return item;
    }

    var einoSubReplyStreams = new Map();
    var wsThinkingStreams = new Map();        // streamId → { el, buf }
    var wsToolResultStreams = new Map();      // toolCallId → { el, buf }

    if (inputEl) inputEl.value = '';

    var convId = webshellAiConvMap[conn.id] || '';
    var wsRole = (typeof getCurrentRole === 'function') ? getCurrentRole() : (localStorage.getItem('currentRole') || '');
    var body = {
        message: message,
        webshellConnectionId: conn.id,
        conversationId: convId,
        role: wsRole
    };

    // English note.
    var streamingTarget = '';  // （）
    var streamingTypingId = 0;  // ， response 

    resolveWebshellAiStreamRequest().then(function (info) {
        if (info && info.orchestration) {
            body.orchestration = info.orchestration;
        }
        return apiFetch(info.path, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
            signal: webshellAiAbortController ? webshellAiAbortController.signal : undefined
        });
    }).then(function (response) {
        if (!response.ok) {
            renderWebshellAiErrorMessage(assistantDiv, ': HTTP ' + response.status);
            return;
        }
        return response.body.getReader();
    }).then(function (reader) {
        if (!reader) return;
        webshellAiStreamReader = reader;
        var decoder = new TextDecoder();
        var buffer = '';
        return reader.read().then(function processChunk(result) {
            if (result.done) return;
            buffer += decoder.decode(result.value, { stream: true });
            var lines = buffer.split('\n');
            buffer = lines.pop() || '';
            for (var i = 0; i < lines.length; i++) {
                var line = lines[i];
                if (line.indexOf('data: ') !== 0) continue;
                try {
                    var eventData = JSON.parse(line.slice(6));
                    var _et = eventData.type;
                    var _ed = eventData.data || {};
                    var _em = eventData.message || '';

                    if (_et === 'conversation' && _ed.conversationId) {
                        var convId = _ed.conversationId;
                        webshellAiConvMap[conn.id] = convId;
                        var listEl = document.getElementById('webshell-ai-conv-list');
                        if (listEl) fetchAndRenderWebshellAiConvList(conn, listEl).then(function () {
                            listEl.querySelectorAll('.webshell-ai-conv-item').forEach(function (el) {
                                el.classList.toggle('active', el.dataset.convId === convId);
                            });
                        });

                    // ─── Response streaming ───
                    } else if (_et === 'response_start') {
                        streamingTarget = '';
                        webshellStreamingTypingId += 1;
                        streamingTypingId = webshellStreamingTypingId;
                        assistantDiv.textContent = '…';
                        messagesContainer.scrollTop = messagesContainer.scrollHeight;
                    } else if (_et === 'response_delta') {
                        var deltaText = (_em != null && _em !== '') ? String(_em) : '';
                        if (deltaText) {
                            streamingTarget += deltaText;
                            webshellStreamingTypingId += 1;
                            streamingTypingId = webshellStreamingTypingId;
                            runWebshellAiStreamingTyping(assistantDiv, streamingTarget, streamingTypingId, messagesContainer);
                        }
                    } else if (_et === 'response') {
                        var text = (_em != null && _em !== '') ? _em : (typeof _ed === 'string' ? _ed : '');
                        if (text) {
                            streamingTarget = String(text);
                            webshellStreamingTypingId += 1;
                            streamingTypingId = webshellStreamingTypingId;
                            runWebshellAiStreamingTyping(assistantDiv, streamingTarget, streamingTypingId, messagesContainer);
                        }

                    // ─── Terminal events ───
                    } else if (_et === 'error' && _em) {
                        streamingTypingId += 1;
                        var errLabel = wsTOr('chat.error', '');
                        appendTimelineItem('error', '❌ ' + errLabel, _em, _ed);
                        renderWebshellAiErrorMessage(assistantDiv, errLabel + ': ' + _em);
                    } else if (_et === 'cancelled') {
                        streamingTypingId += 1;
                        var cancelLabel = wsTOr('chat.taskCancelled', '');
                        appendTimelineItem('cancelled', '⛔ ' + cancelLabel, _em, _ed);
                        if (!streamingTarget && !assistantDiv.dataset.hasContent) {
                            assistantDiv.textContent = cancelLabel;
                        }
                    } else if (_et === 'done') {
                        // English note.
                        wsThinkingStreams.clear();
                        wsToolResultStreams.clear();
                        einoSubReplyStreams.clear();

                    // ─── Iteration / Progress ───
                    } else if (_et === 'progress' && _em) {
                        var progressMsg = (typeof window.translateProgressMessage === 'function')
                            ? window.translateProgressMessage(_em) : _em;
                        appendTimelineItem('progress', '🔍 ' + progressMsg, '', _ed);
                        if (!streamingTarget) assistantDiv.textContent = '…';
                    } else if (_et === 'iteration') {
                        var iterN = _ed.iteration || 0;
                        var iterTitle = wsTOr('chat.iterationRound', '') || (iterN ? (' ' + iterN + ' ') : (_em || ''));
                        if (typeof window.t === 'function' && iterN) {
                            iterTitle = window.t('chat.iterationRound', { n: iterN });
                        }
                        var iterMessage = _em || '';
                        if (iterMessage && typeof window.translateProgressMessage === 'function') {
                            iterMessage = window.translateProgressMessage(iterMessage);
                        }
                        appendTimelineItem('iteration', '🔍 ' + iterTitle, iterMessage, _ed);
                        if (!streamingTarget) assistantDiv.textContent = '…';

                    // ─── Thinking (non-stream + stream) ───
                    } else if (_et === 'thinking_stream_start' && _ed.streamId) {
                        var thinkSLabel = wsTOr('chat.aiThinking', 'AI ');
                        var thinkSItem = document.createElement('div');
                        thinkSItem.className = 'webshell-ai-timeline-item webshell-ai-timeline-thinking';
                        thinkSItem.innerHTML = '<span class="webshell-ai-timeline-title">' + escapeHtml(webshellAgentPx(_ed) + '🤔 ' + thinkSLabel) + '</span>';
                        var thinkSPre = document.createElement('div');
                        thinkSPre.className = 'webshell-ai-timeline-msg webshell-thinking-stream-body';
                        thinkSItem.appendChild(thinkSPre);
                        timelineContainer.appendChild(thinkSItem);
                        timelineContainer.classList.add('has-items');
                        wsThinkingStreams.set(_ed.streamId, { el: thinkSItem, body: thinkSPre, buf: '' });
                        if (!streamingTarget) assistantDiv.textContent = '…';
                    } else if (_et === 'thinking_stream_delta' && _ed.streamId) {
                        var tsD = wsThinkingStreams.get(_ed.streamId);
                        if (tsD) {
                            tsD.buf += (_em || '');
                            if (typeof formatMarkdown === 'function') {
                                tsD.body.innerHTML = formatMarkdown(tsD.buf);
                            } else {
                                tsD.body.textContent = tsD.buf;
                            }
                        }
                        if (!streamingTarget) assistantDiv.textContent = '…';
                    } else if (_et === 'thinking_stream_end' && _ed.streamId) {
                        var tsE = wsThinkingStreams.get(_ed.streamId);
                        if (tsE) {
                            var fullThink = (_em != null && _em !== '') ? String(_em) : tsE.buf;
                            if (typeof formatMarkdown === 'function') {
                                tsE.body.innerHTML = formatMarkdown(fullThink);
                            } else {
                                tsE.body.textContent = fullThink;
                            }
                            wsThinkingStreams.delete(_ed.streamId);
                        }
                    } else if (_et === 'thinking' && _em) {
                        // English note.
                        if (_ed.streamId && wsThinkingStreams.has(_ed.streamId)) {
                            // English note.
                        } else {
                            var thinkLabel = wsTOr('chat.aiThinking', 'AI ');
                            appendTimelineItem('thinking', webshellAgentPx(_ed) + '🤔 ' + thinkLabel, _em, _ed);
                        }
                        if (!streamingTarget) assistantDiv.textContent = '…';

                    // ─── Warning ───
                    } else if (_et === 'warning') {
                        appendTimelineItem('warning', '⚠️ ' + (_em || ''), '', _ed);

                    // ─── Eino recovery ───
                    } else if (_et === 'eino_recovery') {
                        var runIdx = _ed.runIndex != null ? _ed.runIndex : (_ed.einoRetry != null ? _ed.einoRetry + 1 : 1);
                        var maxRuns = _ed.maxRuns != null ? _ed.maxRuns : 3;
                        var recTitle = wsTOr('chat.einoRecoveryTitle', '') ||
                            ('🔄  ·  ' + runIdx + '/' + maxRuns + ' （）');
                        if (typeof window.t === 'function') {
                            try { recTitle = window.t('chat.einoRecoveryTitle', { n: runIdx, max: maxRuns }); } catch (e) { /* */ }
                        }
                        appendTimelineItem('eino_recovery', recTitle, _em, _ed);

                    // ─── Tool calls ───
                    } else if (_et === 'tool_calls_detected' && _ed) {
                        var count = _ed.count || 0;
                        var detectedLabel = wsTOr('chat.toolCallsDetected', '') || (' ' + count + ' ');
                        if (typeof window.t === 'function') {
                            try { detectedLabel = window.t('chat.toolCallsDetected', { count: count }); } catch (e) { /* */ }
                        }
                        appendTimelineItem('tool_calls_detected', webshellAgentPx(_ed) + '🔧 ' + detectedLabel, _em || '', _ed);
                        if (!streamingTarget) assistantDiv.textContent = '…';
                    } else if (_et === 'tool_call' && _ed) {
                        var tn = _ed.toolName || '';
                        var idx = _ed.index || 0;
                        var total = _ed.total || 0;
                        var callTitle = wsTOr('chat.callTool', '') || (': ' + tn + (total ? ' (' + idx + '/' + total + ')' : ''));
                        if (typeof window.t === 'function') {
                            try { callTitle = window.t('chat.callTool', { name: tn, index: idx, total: total }); } catch (e) { /* */ }
                        }
                        appendTimelineItem('tool_call', webshellAgentPx(_ed) + '🔧 ' + callTitle, _em || '', _ed);
                        if (!streamingTarget) assistantDiv.textContent = '…';

                    // ─── Tool result delta (streaming output) ───
                    } else if (_et === 'tool_result_delta' && _ed.toolCallId) {
                        var trdKey = _ed.toolCallId;
                        var trdDelta = _em || '';
                        if (trdDelta) {
                            var trdState = wsToolResultStreams.get(trdKey);
                            if (!trdState) {
                                var trdName = _ed.toolName || '';
                                var runLabel = wsTOr('timeline.running', '...');
                                var trdItem = document.createElement('div');
                                trdItem.className = 'webshell-ai-timeline-item webshell-ai-timeline-tool_result';
                                trdItem.innerHTML = '<span class="webshell-ai-timeline-title">' +
                                    escapeHtml(webshellAgentPx(_ed) + '⏳ ' + runLabel + ' ' + trdName) +
                                    '</span><div class="webshell-ai-timeline-msg"><div class="tool-result-section success">' +
                                    '<pre class="tool-result"></pre></div></div>';
                                timelineContainer.appendChild(trdItem);
                                timelineContainer.classList.add('has-items');
                                trdState = { el: trdItem, buf: '' };
                                wsToolResultStreams.set(trdKey, trdState);
                            }
                            trdState.buf += trdDelta;
                            var trdPre = trdState.el.querySelector('pre.tool-result');
                            if (trdPre) trdPre.textContent = trdState.buf;
                        }
                        if (!streamingTarget) assistantDiv.textContent = '…';

                    // ─── Tool result (final) ───
                    } else if (_et === 'tool_result' && _ed) {
                        var success = _ed.success !== false;
                        var tname = _ed.toolName || '';
                        var titleText = wsTOr(success ? 'chat.toolExecComplete' : 'chat.toolExecFailed', '') ||
                            (tname + (success ? ' ' : ' '));
                        if (typeof window.t === 'function') {
                            try { titleText = window.t(success ? 'chat.toolExecComplete' : 'chat.toolExecFailed', { name: tname }); } catch (e) { /* */ }
                        }
                        // English note.
                        var trdExist = _ed.toolCallId ? wsToolResultStreams.get(_ed.toolCallId) : null;
                        if (trdExist) {
                            var trdTitleEl = trdExist.el.querySelector('.webshell-ai-timeline-title');
                            if (trdTitleEl) trdTitleEl.textContent = webshellAgentPx(_ed) + (success ? '✅ ' : '❌ ') + titleText;
                            // English note.
                            var resultText = _ed.result ? String(_ed.result) : (_em || '');
                            var trdPreEl = trdExist.el.querySelector('pre.tool-result');
                            if (trdPreEl && resultText) trdPreEl.textContent = resultText;
                            // English note.
                            var trdSection = trdExist.el.querySelector('.tool-result-section');
                            if (trdSection) { trdSection.className = 'tool-result-section ' + (success ? 'success' : 'error'); }
                            wsToolResultStreams.delete(_ed.toolCallId);
                        } else {
                            var title = webshellAgentPx(_ed) + (success ? '✅ ' : '❌ ') + titleText;
                            var sub = _em || (_ed.result ? String(_ed.result).slice(0, 300) : '');
                            appendTimelineItem('tool_result', title, sub, _ed);
                        }
                        if (!streamingTarget) assistantDiv.textContent = '…';

                    // ─── Eino sub-agent reply streaming ───
                    } else if (_et === 'eino_agent_reply_stream_start' && _ed.streamId) {
                        var repTS = wsTOr('chat.einoAgentReplyTitle', '');
                        var runTS = wsTOr('timeline.running', '...');
                        var itemS = document.createElement('div');
                        itemS.className = 'webshell-ai-timeline-item webshell-ai-timeline-eino_agent_reply';
                        itemS.innerHTML = '<span class="webshell-ai-timeline-title">' + escapeHtml(webshellAgentPx(_ed) + '💬 ' + repTS + ' · ' + runTS) + '</span>';
                        timelineContainer.appendChild(itemS);
                        timelineContainer.classList.add('has-items');
                        einoSubReplyStreams.set(_ed.streamId, { el: itemS, buf: '' });
                        if (!streamingTarget) assistantDiv.textContent = '…';
                    } else if (_et === 'eino_agent_reply_stream_delta' && _ed.streamId) {
                        var stD = einoSubReplyStreams.get(_ed.streamId);
                        if (stD) {
                            stD.buf += (_em || '');
                            var preD = stD.el.querySelector('.webshell-eino-reply-stream-body');
                            if (!preD) {
                                preD = document.createElement('pre');
                                preD.className = 'webshell-ai-timeline-msg webshell-eino-reply-stream-body';
                                preD.style.whiteSpace = 'pre-wrap';
                                stD.el.appendChild(preD);
                            }
                            if (typeof formatMarkdown === 'function') {
                                preD.innerHTML = formatMarkdown(stD.buf);
                            } else {
                                preD.textContent = stD.buf;
                            }
                        }
                        if (!streamingTarget) assistantDiv.textContent = '…';
                    } else if (_et === 'eino_agent_reply_stream_end' && _ed.streamId) {
                        var stE = einoSubReplyStreams.get(_ed.streamId);
                        if (stE) {
                            var fullE = (_em != null && _em !== '') ? String(_em) : stE.buf;
                            var repTE = wsTOr('chat.einoAgentReplyTitle', '');
                            var titE = stE.el.querySelector('.webshell-ai-timeline-title');
                            if (titE) titE.textContent = webshellAgentPx(_ed) + '💬 ' + repTE;
                            var preE = stE.el.querySelector('.webshell-eino-reply-stream-body');
                            if (!preE) {
                                preE = document.createElement('pre');
                                preE.className = 'webshell-ai-timeline-msg webshell-eino-reply-stream-body';
                                preE.style.whiteSpace = 'pre-wrap';
                                stE.el.appendChild(preE);
                            }
                            if (typeof formatMarkdown === 'function') {
                                preE.innerHTML = formatMarkdown(fullE);
                            } else {
                                preE.textContent = fullE;
                            }
                            einoSubReplyStreams.delete(_ed.streamId);
                        }
                        if (!streamingTarget) assistantDiv.textContent = '…';
                    } else if (_et === 'eino_agent_reply' && _em) {
                        var replyT = wsTOr('chat.einoAgentReplyTitle', '');
                        appendTimelineItem('eino_agent_reply', webshellAgentPx(_ed) + '💬 ' + replyT, _em, _ed);
                        if (!streamingTarget) assistantDiv.textContent = '…';
                    }
                } catch (e) { /* ignore parse error */ }
            }
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
            return reader.read().then(processChunk);
        });
    }).catch(function (err) {
        var msg = err && err.message ? err.message : String(err);
        var isAbort = /abort/i.test(msg);
        if (!isAbort) {
            renderWebshellAiErrorMessage(assistantDiv, ': ' + msg);
        }
    }).then(function () {
        webshellAiAbortController = null;
        webshellAiStreamReader = null;
        wsSetAiSendingState(false);
        if (assistantDiv.textContent === '…' && !streamingTarget) {
            // English note.
            assistantDiv.textContent = '';
        } else if (streamingTarget) {
            // English note.
            webshellStreamingTypingId += 1;
            // English note.
            if (typeof formatMarkdown === 'function') {
                assistantDiv.innerHTML = formatMarkdown(streamingTarget);
            } else {
                assistantDiv.textContent = streamingTarget;
            }
        }
        // English note.
        if (timelineContainer && timelineContainer.classList.contains('has-items') && !timelineContainer.closest('.webshell-ai-process-block')) {
            var headerLabel = (typeof window.t === 'function') ? (window.t('chat.penetrationTestDetail') || '') : '';
            var wrap = document.createElement('div');
            wrap.className = 'process-details-container webshell-ai-process-block';
            wrap.innerHTML = '<button type="button" class="webshell-ai-process-toggle" aria-expanded="false">' + escapeHtml(headerLabel) + ' <span class="ws-toggle-icon">▶</span></button><div class="process-details-content"></div>';
            var contentDiv = wrap.querySelector('.process-details-content');
            contentDiv.appendChild(timelineContainer);
            timelineContainer.classList.add('progress-timeline');
            messagesContainer.insertBefore(wrap, assistantDiv.nextSibling);
            var toggleBtn = wrap.querySelector('.webshell-ai-process-toggle');
            var toggleIcon = wrap.querySelector('.ws-toggle-icon');
            toggleBtn.addEventListener('click', function () {
                var isExpanded = timelineContainer.classList.contains('expanded');
                timelineContainer.classList.toggle('expanded');
                toggleBtn.setAttribute('aria-expanded', !isExpanded);
                if (toggleIcon) toggleIcon.textContent = isExpanded ? '▶' : '▼';
            });
        }
        messagesContainer.scrollTop = messagesContainer.scrollHeight;
    });
}

// English note.
function runWebshellAiStreamingTyping(el, target, id, scrollContainer) {
    if (!el || id === undefined) return;
    var chunkSize = 3;
    var delayMs = 24;
    function tick() {
        if (id !== webshellStreamingTypingId) return;
        var cur = el.textContent || '';
        if (cur.length >= target.length) {
            el.textContent = target;
            if (scrollContainer) scrollContainer.scrollTop = scrollContainer.scrollHeight;
            return;
        }
        var next = target.slice(0, cur.length + chunkSize);
        el.textContent = next;
        if (scrollContainer) scrollContainer.scrollTop = scrollContainer.scrollHeight;
        setTimeout(tick, delayMs);
    }
    if (el.textContent.length < target.length) setTimeout(tick, delayMs);
}

function getWebshellHistory(connId) {
    if (!connId) return [];
    if (!webshellHistoryByConn[connId]) webshellHistoryByConn[connId] = [];
    return webshellHistoryByConn[connId];
}
function pushWebshellHistory(connId, cmd) {
    if (!connId || !cmd) return;
    if (!webshellHistoryByConn[connId]) webshellHistoryByConn[connId] = [];
    var h = webshellHistoryByConn[connId];
    if (h[h.length - 1] === cmd) return;
    h.push(cmd);
    if (h.length > WEBSHELL_HISTORY_MAX) h.shift();
}

// English note.
function runQuickCommand(cmd) {
    if (!webshellCurrentConn || !webshellTerminalInstance) return;
    if (webshellRunning || webshellTerminalRunning) return;
    var term = webshellTerminalInstance;
    var connId = webshellCurrentConn.id;
    var sessionId = getActiveWebshellTerminalSessionId(connId);
    var terminalKey = getWebshellTerminalSessionKey(connId, sessionId);
    term.writeln('');
    pushWebshellHistory(terminalKey, cmd);
    appendWebshellTerminalLog(terminalKey, '\n$ ' + cmd + '\n');
    webshellRunning = true;
    setWebshellTerminalStatus(true);
    execWebshellCommand(webshellCurrentConn, cmd).then(function (out) {
        var s = String(out || '').replace(/\r\n/g, '\n').replace(/\r/g, '\n');
        s.split('\n').forEach(function (line) { term.writeln(line.replace(/\r/g, '')); });
        appendWebshellTerminalLog(terminalKey, s + '\n');
        term.write(WEBSHELL_PROMPT);
    }).catch(function (err) {
        var em = (err && err.message ? err.message : wsT('webshell.execError'));
        term.writeln('\x1b[31m' + em + '\x1b[0m');
        appendWebshellTerminalLog(terminalKey, em + '\n');
        term.write(WEBSHELL_PROMPT);
    }).finally(function () {
        webshellRunning = false;
        setWebshellTerminalStatus(false);
        renderWebshellTerminalSessions(webshellCurrentConn);
    });
}

// English note.
function initWebshellTerminal(conn) {
    const container = document.getElementById('webshell-terminal-container');
    if (!container || typeof Terminal === 'undefined') {
        if (container) {
            container.innerHTML = '<p class="terminal-error">' + escapeHtml(' xterm.js，') + '</p>';
        }
        return;
    }

    const term = new Terminal({
        cursorBlink: true,
        cursorStyle: 'underline',
        fontSize: 13,
        fontFamily: 'Menlo, Monaco, "Courier New", monospace',
        lineHeight: 1.2,
        scrollback: 2000,
        theme: {
            background: '#0d1117',
            foreground: '#e6edf3',
            cursor: '#58a6ff',
            cursorAccent: '#0d1117',
            selection: 'rgba(88, 166, 255, 0.3)'
        }
    });

    let fitAddon = null;
    if (typeof FitAddon !== 'undefined') {
        const FitCtor = FitAddon.FitAddon || FitAddon;
        fitAddon = new FitCtor();
        term.loadAddon(fitAddon);
    }

    term.open(container);
    // English note.
    try {
        if (fitAddon) fitAddon.fit();
    } catch (e) {}
    setWebshellTerminalStatus(false);
    var connId = conn && conn.id ? conn.id : '';
    var sessionId = getActiveWebshellTerminalSessionId(connId);
    var terminalKey = getWebshellTerminalSessionKey(connId, sessionId);
    var cachedLog = getWebshellTerminalLog(terminalKey);
    if (cachedLog) {
        // English note.
        term.write(String(cachedLog).replace(/\r\n/g, '\n').replace(/\r/g, '\n').replace(/\n/g, '\r\n'));
    }
    term.write(WEBSHELL_PROMPT);

    // English note.
    function writeWebshellOutput(term, text, isError) {
        if (!term || !text) return;
        var s = String(text).replace(/\r\n/g, '\n').replace(/\r/g, '\n');
        var lines = s.split('\n');
        var prefix = isError ? '\x1b[31m' : '';
        var suffix = isError ? '\x1b[0m' : '';
        term.write(prefix);
        for (var i = 0; i < lines.length; i++) {
            term.writeln(lines[i].replace(/\r/g, ''));
        }
        term.write(suffix);
    }

    term.onData(function (data) {
        // English note.
        if (data === '\x0c') {
            term.clear();
            webshellLineBuffer = '';
            webshellHistoryIndex = -1;
            term.write(WEBSHELL_PROMPT);
            clearWebshellTerminalLog(terminalKey);
            return;
        }
        // English note.
        if (data === '\x03') {
            if (webshellTerminalRunning) {
                writeWebshellOutput(term, '^C ()', true);
                appendWebshellTerminalLog(terminalKey, '^C ()\n');
            }
            webshellLineBuffer = '';
            webshellHistoryIndex = -1;
            term.write(WEBSHELL_PROMPT);
            return;
        }
        // English note.
        if (data === '\x15') {
            webshellLineBuffer = '';
            term.write('\x1b[2K\r' + WEBSHELL_PROMPT);
            return;
        }
        // English note.
        if (data === '\x1b[A' || data === '\x1bOA') {
            var hist = getWebshellHistory(terminalKey);
            if (hist.length === 0) return;
            webshellHistoryIndex = webshellHistoryIndex < 0 ? hist.length : Math.max(0, webshellHistoryIndex - 1);
            webshellLineBuffer = hist[webshellHistoryIndex] || '';
            term.write('\x1b[2K\r' + WEBSHELL_PROMPT + webshellLineBuffer);
            return;
        }
        if (data === '\x1b[B' || data === '\x1bOB') {
            var hist2 = getWebshellHistory(terminalKey);
            if (hist2.length === 0) return;
            webshellHistoryIndex = webshellHistoryIndex < 0 ? -1 : Math.min(hist2.length - 1, webshellHistoryIndex + 1);
            if (webshellHistoryIndex < 0) webshellLineBuffer = '';
            else webshellLineBuffer = hist2[webshellHistoryIndex] || '';
            term.write('\x1b[2K\r' + WEBSHELL_PROMPT + webshellLineBuffer);
            return;
        }
        // English note.
        if (data === '\r' || data === '\n') {
            term.writeln('');
            var cmd = webshellLineBuffer.trim();
            webshellLineBuffer = '';
            webshellHistoryIndex = -1;
            if (cmd) {
                if (webshellRunning) {
                    writeWebshellOutput(term, wsT('webshell.waitFinish'), true);
                    appendWebshellTerminalLog(terminalKey, (wsT('webshell.waitFinish') || '') + '\n');
                    term.write(WEBSHELL_PROMPT);
                    return;
                }
                pushWebshellHistory(terminalKey, cmd);
                appendWebshellTerminalLog(terminalKey, '$ ' + cmd + '\n');
                webshellRunning = true;
                setWebshellTerminalStatus(true);
                renderWebshellTerminalSessions(conn);
                execWebshellCommand(webshellCurrentConn, cmd).then(function (out) {
                    webshellRunning = false;
                    setWebshellTerminalStatus(false);
                    renderWebshellTerminalSessions(conn);
                    if (out && out.length) {
                        writeWebshellOutput(term, out, false);
                        appendWebshellTerminalLog(terminalKey, String(out).replace(/\r\n/g, '\n').replace(/\r/g, '\n') + '\n');
                    }
                    term.write(WEBSHELL_PROMPT);
                }).catch(function (err) {
                    webshellRunning = false;
                    setWebshellTerminalStatus(false);
                    renderWebshellTerminalSessions(conn);
                    var errMsg = err && err.message ? err.message : wsT('webshell.execError');
                    writeWebshellOutput(term, errMsg, true);
                    appendWebshellTerminalLog(terminalKey, String(errMsg || '') + '\n');
                    term.write(WEBSHELL_PROMPT);
                });
            } else {
                term.write(WEBSHELL_PROMPT);
            }
            return;
        }
        // English note.
        if (data.indexOf('\n') !== -1 || data.indexOf('\r') !== -1) {
            var full = (webshellLineBuffer + data).replace(/\r\n/g, '\n').replace(/\r/g, '\n');
            var lines = full.split('\n');
            webshellLineBuffer = lines.pop() || '';
            if (lines.length > 0 && !webshellRunning && webshellCurrentConn) {
                var runNext = function (idx) {
                    if (idx >= lines.length) {
                        term.write(WEBSHELL_PROMPT + webshellLineBuffer);
                        return;
                    }
                    var line = lines[idx].trim();
                    if (!line) { runNext(idx + 1); return; }
                    pushWebshellHistory(terminalKey, line);
                    appendWebshellTerminalLog(terminalKey, '$ ' + line + '\n');
                    webshellRunning = true;
                    setWebshellTerminalStatus(true);
                    renderWebshellTerminalSessions(conn);
                    execWebshellCommand(webshellCurrentConn, line).then(function (out) {
                        if (out && out.length) {
                            writeWebshellOutput(term, out, false);
                            appendWebshellTerminalLog(terminalKey, String(out).replace(/\r\n/g, '\n').replace(/\r/g, '\n') + '\n');
                        }
                        webshellRunning = false;
                        setWebshellTerminalStatus(false);
                        renderWebshellTerminalSessions(conn);
                        runNext(idx + 1);
                    }).catch(function (err) {
                        var em = err && err.message ? err.message : wsT('webshell.execError');
                        writeWebshellOutput(term, em, true);
                        appendWebshellTerminalLog(terminalKey, String(em || '') + '\n');
                        webshellRunning = false;
                        setWebshellTerminalStatus(false);
                        renderWebshellTerminalSessions(conn);
                        runNext(idx + 1);
                    });
                };
                runNext(0);
            } else {
                term.write(data);
            }
            return;
        }
        // English note.
        if (data === '\x7f' || data === '\b') {
            if (webshellLineBuffer.length > 0) {
                webshellLineBuffer = webshellLineBuffer.slice(0, -1);
                term.write('\b \b');
            }
            return;
        }
        webshellLineBuffer += data;
        term.write(data);
    });

    webshellTerminalInstance = term;
    webshellTerminalFitAddon = fitAddon;
    // English note.
    setTimeout(function () {
        try { if (fitAddon) fitAddon.fit(); } catch (e) {}
    }, 100);
    // English note.
    if (fitAddon && typeof ResizeObserver !== 'undefined' && container) {
        webshellTerminalResizeContainer = container;
        webshellTerminalResizeObserver = new ResizeObserver(function () {
            try { fitAddon.fit(); } catch (e) {}
        });
        webshellTerminalResizeObserver.observe(container);
    }
    renderWebshellTerminalSessions(conn);
}

// English note.
function execWebshellCommand(conn, command) {
    return new Promise(function (resolve, reject) {
        if (typeof apiFetch === 'undefined') {
            reject(new Error('apiFetch '));
            return;
        }
        apiFetch('/api/webshell/exec', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                url: conn.url,
                password: conn.password || '',
                type: conn.type || 'php',
                method: (conn.method || 'post').toLowerCase(),
                cmd_param: conn.cmdParam || '',
                command: command
            })
        }).then(function (r) { return r.json(); })
            .then(function (data) {
                if (data && data.output !== undefined) resolve(data.output || '');
                else if (data && data.error) reject(new Error(data.error));
                else resolve('');
            })
            .catch(reject);
    });
}

// English note.
function webshellFileListDir(conn, path) {
    const listEl = document.getElementById('webshell-file-list');
    if (!listEl) return;
    listEl.innerHTML = '<div class="webshell-loading">' + wsT('common.refresh') + '...</div>';

    if (typeof apiFetch === 'undefined') {
        listEl.innerHTML = '<div class="webshell-file-error">apiFetch </div>';
        return;
    }

    apiFetch('/api/webshell/file', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            url: conn.url,
            password: conn.password || '',
            type: conn.type || 'php',
            method: (conn.method || 'post').toLowerCase(),
            cmd_param: conn.cmdParam || '',
            action: 'list',
            path: path
        })
    }).then(function (r) { return r.json(); })
        .then(function (data) {
            if (!data.ok && data.error) {
                listEl.innerHTML = '<div class="webshell-file-error">' + escapeHtml(data.error) + '</div><pre class="webshell-file-raw">' + escapeHtml(data.output || '') + '</pre>';
                return;
            }
            listEl.dataset.currentPath = path;
            listEl.dataset.rawOutput = data.output || '';
            renderFileList(listEl, path, data.output || '', conn);
        })
        .catch(function (err) {
            listEl.innerHTML = '<div class="webshell-file-error">' + escapeHtml(err && err.message ? err.message : wsT('webshell.execError')) + '</div>';
        });
}

function normalizeLsMtime(month, day, timeOrYear) {
    if (!month || !day || !timeOrYear) return '';
    var token = String(timeOrYear).trim();
    if (/^\d{4}$/.test(token)) return token + ' ' + month + ' ' + day;
    var now = new Date();
    var year = now.getFullYear();
    if (/^\d{1,2}:\d{2}$/.test(token)) {
        var monthMap = { Jan: 0, Feb: 1, Mar: 2, Apr: 3, May: 4, Jun: 5, Jul: 6, Aug: 7, Sep: 8, Oct: 9, Nov: 10, Dec: 11 };
        var m = monthMap[month];
        var d = parseInt(day, 10);
        if (m != null && !isNaN(d)) {
            var inferred = new Date(year, m, d);
            if (inferred.getTime() > now.getTime()) year = year - 1;
        }
        return year + ' ' + month + ' ' + day + ' ' + token;
    }
    return month + ' ' + day + ' ' + token;
}

function modeToType(mode) {
    if (!mode || !mode.length) return '';
    var c = mode.charAt(0);
    if (c === 'd') return 'dir';
    if (c === '-') return 'file';
    if (c === 'l') return 'link';
    if (c === 'c') return 'char';
    if (c === 'b') return 'block';
    if (c === 's') return 'socket';
    if (c === 'p') return 'pipe';
    return c;
}

function parseWebshellListItems(rawOutput) {
    var lines = (rawOutput || '').split(/\n/).filter(function (l) { return l.trim(); });
    var items = [];
    for (var i = 0; i < lines.length; i++) {
        var line = lines[i];
        var trimmedLine = String(line || '').trim();
        // English note.
        if (/^(total|)\s+\d+$/i.test(trimmedLine)) continue;
        var name = '';
        var isDir = false;
        var size = '';
        var mode = '';
        var mtime = '';
        var owner = '';
        var group = '';
        var type = '';
        var mLs = line.match(/^(\S+)\s+(\d+)\s+(\S+)\s+(\S+)\s+(\d+)\s+([A-Za-z]{3})\s+(\d{1,2})\s+(\S+)\s+(.+)$/);
        if (mLs) {
            mode = mLs[1];
            owner = mLs[3];
            group = mLs[4];
            size = mLs[5];
            mtime = normalizeLsMtime(mLs[6], mLs[7], mLs[8]);
            name = (mLs[9] || '').trim();
            isDir = mode && mode.startsWith('d');
            type = modeToType(mode);
        } else {
            var mName = line.match(/\s*(\S+)\s*$/);
            name = mName ? mName[1].trim() : line.trim();
            if (name === '.' || name === '..') continue;
            isDir = line.startsWith('d') || line.toLowerCase().indexOf('<dir>') !== -1;
            if (line.startsWith('-') || line.startsWith('d')) {
                var parts = line.split(/\s+/);
                if (parts.length >= 5) { mode = parts[0]; size = parts[4]; }
                if (parts.length >= 4) { owner = parts[2] || ''; group = parts[3] || ''; }
                if (parts.length >= 8 && /^[A-Za-z]{3}$/.test(parts[5])) mtime = normalizeLsMtime(parts[5], parts[6], parts[7]);
                type = modeToType(mode);
            }
        }
        if (name === '.' || name === '..') continue;
        items.push({ name: name, isDir: isDir, line: line, size: size, mode: mode, mtime: mtime, owner: owner, group: group, type: type });
    }
    return items;
}

function fetchWebshellDirectoryItems(conn, path) {
    if (!conn || typeof apiFetch === 'undefined') return Promise.resolve([]);
    return apiFetch('/api/webshell/file', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            url: conn.url,
            password: conn.password || '',
            type: conn.type || 'php',
            method: (conn.method || 'post').toLowerCase(),
            cmd_param: conn.cmdParam || '',
            action: 'list',
            path: path
        })
    }).then(function (r) { return r.json(); }).then(function (data) {
        if (!data || data.error || !data.ok) return [];
        return parseWebshellListItems(data.output || '');
    }).catch(function () {
        return [];
    });
}

function renderFileList(listEl, currentPath, rawOutput, conn, nameFilter) {
    var items = parseWebshellListItems(rawOutput);
    if (nameFilter && nameFilter.trim()) {
        var f = nameFilter.trim().toLowerCase();
        items = items.filter(function (item) { return item.name.toLowerCase().indexOf(f) !== -1; });
    }
    // English note.
    var breadcrumbEl = document.getElementById('webshell-file-breadcrumb');
    if (breadcrumbEl) {
        var parts = (currentPath === '.' || currentPath === '') ? [] : currentPath.replace(/^\//, '').split('/');
        breadcrumbEl.innerHTML = '<a href="#" class="webshell-breadcrumb-item" data-path=".">' + (wsT('webshell.breadcrumbHome') || '') + '</a>' +
            parts.map(function (p, idx) {
                var path = parts.slice(0, idx + 1).join('/');
                return ' / <a href="#" class="webshell-breadcrumb-item" data-path="' + escapeHtml(path) + '">' + escapeHtml(p) + '</a>';
            }).join('');
    }
    renderDirectoryTree(currentPath, items, conn);
    var html = '';
    if (items.length === 0) {
        // English note.
        if (rawOutput.trim() && !nameFilter) {
            html = '<pre class="webshell-file-raw">' + escapeHtml(rawOutput) + '</pre>';
        } else {
            html = '<table class="webshell-file-table"><thead><tr><th class="webshell-col-check"><input type="checkbox" id="webshell-file-select-all" title="' + (wsT('webshell.selectAll') || '') + '" /></th><th>' + wsT('webshell.filePath') + '</th><th class="webshell-col-size"></th><th class="webshell-col-mtime">' + (wsT('webshell.colModifiedAt') || '') + '</th><th class="webshell-col-owner">' + (wsT('webshell.colOwner') || '') + '</th><th class="webshell-col-perms">' + (wsT('webshell.colPerms') || '') + '</th><th class="webshell-col-actions"></th></tr></thead><tbody>' +
                '<tr><td colspan="7" class="webshell-file-empty-state">' + (wsT('common.noData') || '') + '</td></tr>' +
                '</tbody></table>';
        }
    } else {
        html = '<table class="webshell-file-table"><thead><tr><th class="webshell-col-check"><input type="checkbox" id="webshell-file-select-all" title="' + (wsT('webshell.selectAll') || '') + '" /></th><th>' + wsT('webshell.filePath') + '</th><th class="webshell-col-size"></th><th class="webshell-col-mtime">' + (wsT('webshell.colModifiedAt') || '') + '</th><th class="webshell-col-owner">' + (wsT('webshell.colOwner') || '') + '</th><th class="webshell-col-perms">' + (wsT('webshell.colPerms') || '') + '</th><th class="webshell-col-actions"></th></tr></thead><tbody>';
        if (currentPath !== '.' && currentPath !== '') {
            html += '<tr><td></td><td><a href="#" class="webshell-file-link" data-path="' + escapeHtml(currentPath.replace(/\/[^/]+$/, '') || '.') + '" data-isdir="1">..</a></td><td></td><td></td><td></td><td></td><td></td></tr>';
        }
        items.forEach(function (item) {
            var pathNext = currentPath === '.' ? item.name : currentPath + '/' + item.name;
            var nameClass = item.isDir ? 'is-dir' : 'is-file';
            html += '<tr><td class="webshell-col-check">';
            if (!item.isDir) html += '<input type="checkbox" class="webshell-file-cb" data-path="' + escapeHtml(pathNext) + '" />';
            html += '</td><td><a href="#" class="webshell-file-link ' + nameClass + '" data-path="' + escapeHtml(pathNext) + '" data-isdir="' + (item.isDir ? '1' : '0') + '">' + escapeHtml(item.name) + (item.isDir ? '/' : '') + '</a></td>';
            html += '<td class="webshell-col-size">' + escapeHtml(item.size) + '</td>';
            html += '<td class="webshell-col-mtime">' + escapeHtml(item.mtime || '') + '</td>';
            html += '<td class="webshell-col-owner">' + escapeHtml(item.owner || '') + '</td>';
            html += '<td class="webshell-col-perms">' + escapeHtml(item.mode || '') + '</td>';
            html += '<td class="webshell-col-actions">';
            if (item.isDir) {
                html += '<button type="button" class="btn-ghost btn-sm webshell-file-rename" data-path="' + escapeHtml(pathNext) + '" data-name="' + escapeHtml(item.name) + '">' + (wsT('webshell.rename') || '') + '</button>';
            } else {
                var actionsLabel = wsT('common.actions') || '';
                html += '<details class="webshell-row-actions"><summary class="btn-ghost btn-sm webshell-row-actions-btn" title="' + actionsLabel + '">' + actionsLabel + '</summary>' +
                    '<div class="webshell-row-actions-menu">' +
                    '<button type="button" class="btn-ghost btn-sm webshell-file-read" data-path="' + escapeHtml(pathNext) + '">' + wsT('webshell.readFile') + '</button>' +
                    '<button type="button" class="btn-ghost btn-sm webshell-file-download" data-path="' + escapeHtml(pathNext) + '">' + wsT('webshell.downloadFile') + '</button>' +
                    '<button type="button" class="btn-ghost btn-sm webshell-file-edit" data-path="' + escapeHtml(pathNext) + '">' + wsT('webshell.editFile') + '</button>' +
                    '<button type="button" class="btn-ghost btn-sm webshell-file-rename" data-path="' + escapeHtml(pathNext) + '" data-name="' + escapeHtml(item.name) + '">' + (wsT('webshell.rename') || '') + '</button>' +
                    '<button type="button" class="btn-ghost btn-sm webshell-file-del" data-path="' + escapeHtml(pathNext) + '">' + wsT('webshell.deleteFile') + '</button>' +
                    '</div></details>';
            }
            html += '</td></tr>';
        });
        html += '</tbody></table>';
    }
    listEl.innerHTML = html;

    listEl.querySelectorAll('.webshell-file-link').forEach(function (a) {
        a.addEventListener('click', function (e) {
            e.preventDefault();
            const path = a.getAttribute('data-path');
            const isDir = a.getAttribute('data-isdir') === '1';
            const pathInput = document.getElementById('webshell-file-path');
            if (isDir) {
                if (pathInput) pathInput.value = path;
                webshellFileListDir(webshellCurrentConn, path);
            } else {
                // English note.
                webshellFileRead(webshellCurrentConn, path, listEl, currentPath);
            }
        });
    });
    listEl.querySelectorAll('.webshell-file-read').forEach(function (btn) {
        btn.addEventListener('click', function (e) {
            e.preventDefault();
            webshellFileRead(webshellCurrentConn, btn.getAttribute('data-path'), listEl, currentPath);
        });
    });
    listEl.querySelectorAll('.webshell-file-download').forEach(function (btn) {
        btn.addEventListener('click', function (e) {
            e.preventDefault();
            webshellFileDownload(webshellCurrentConn, btn.getAttribute('data-path'));
        });
    });
    listEl.querySelectorAll('.webshell-file-edit').forEach(function (btn) {
        btn.addEventListener('click', function (e) {
            e.preventDefault();
            webshellFileEdit(webshellCurrentConn, btn.getAttribute('data-path'), listEl);
        });
    });
    listEl.querySelectorAll('.webshell-file-del').forEach(function (btn) {
        btn.addEventListener('click', function (e) {
            e.preventDefault();
            if (!confirm(wsT('webshell.deleteConfirm'))) return;
            webshellFileDelete(webshellCurrentConn, btn.getAttribute('data-path'), function () {
                webshellFileListDir(webshellCurrentConn, document.getElementById('webshell-file-path').value.trim() || '.');
            });
        });
    });
    listEl.querySelectorAll('.webshell-file-rename').forEach(function (btn) {
        btn.addEventListener('click', function (e) {
            e.preventDefault();
            webshellFileRename(webshellCurrentConn, btn.getAttribute('data-path'), btn.getAttribute('data-name'), listEl);
        });
    });
    var selectAll = document.getElementById('webshell-file-select-all');
    if (selectAll) {
        selectAll.addEventListener('change', function () {
            listEl.querySelectorAll('.webshell-file-cb').forEach(function (cb) { cb.checked = selectAll.checked; });
        });
    }
    if (breadcrumbEl) {
        breadcrumbEl.querySelectorAll('.webshell-breadcrumb-item').forEach(function (a) {
            a.addEventListener('click', function (e) {
                e.preventDefault();
                var p = a.getAttribute('data-path');
                var pathInput = document.getElementById('webshell-file-path');
                if (pathInput) pathInput.value = p;
                webshellFileListDir(webshellCurrentConn, p);
            });
        });
    }
}

function renderDirectoryTree(currentPath, items, conn) {
    var treeEl = document.getElementById('webshell-dir-tree');
    if (!treeEl) return;
    var state = getWebshellTreeState(conn || webshellCurrentConn);
    var curr = normalizeWebshellPath(currentPath);
    var dirs = (items || []).filter(function (item) { return item && item.isDir; });
    if (!state) {
        treeEl.innerHTML = '<div class="webshell-empty"></div>';
        return;
    }
    var tree = state.tree;
    var expanded = state.expanded;
    var loaded = state.loaded;
    if (!tree['.']) tree['.'] = [];
    if (expanded['.'] !== false) expanded['.'] = true;

    // English note.
    var childNodes = (items || []).map(function (item) {
        var childPath = curr === '.' ? normalizeWebshellPath(item.name) : normalizeWebshellPath(curr + '/' + item.name);
        return {
            path: childPath,
            name: item.name,
            isDir: !!item.isDir
        };
    }).filter(function (n) { return !!n.path; });
    childNodes.sort(function (a, b) {
        // English note.
        if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
        return (a.name || '').localeCompare(b.name || '');
    });
    tree[curr] = childNodes;
    loaded[curr] = true;
    childNodes.forEach(function (node) {
        if (node.isDir && !tree[node.path]) tree[node.path] = [];
    });

    // English note.
    var parts = curr === '.' ? [] : curr.split('/');
    var parentPath = '.';
    for (var i = 0; i < parts.length; i++) {
        var nextPath = parentPath === '.' ? parts[i] : parentPath + '/' + parts[i];
        if (!tree[parentPath]) tree[parentPath] = [];
        var parentChildren = tree[parentPath];
        var hasAncestorNode = parentChildren.some(function (n) { return n && n.path === nextPath; });
        if (!hasAncestorNode) {
            parentChildren.push({ path: nextPath, name: parts[i], isDir: true });
            parentChildren.sort(function (a, b) {
                if (!!a.isDir !== !!b.isDir) return a.isDir ? -1 : 1;
                return (a.name || '').localeCompare(b.name || '');
            });
        }
        if (!tree[nextPath]) tree[nextPath] = [];
        expanded[parentPath] = true;
        parentPath = nextPath;
    }
    expanded[curr] = true;

    function renderNode(node, depth) {
        var path = node.path;
        var isDir = !!node.isDir;
        var children = isDir ? (tree[path] || []).slice() : [];
        var hasLoadedChildren = isDir ? (loaded[path] === true) : true;
        var canExpand = isDir && (path === '.' || !hasLoadedChildren || children.length > 0);
        var hasChildren = children.length > 0;
        var isExpanded = isDir ? (expanded[path] !== false) : false;
        var isActive = path === curr;
        var name = node.name;
        var icon = isDir ? (path === '.' ? '🗂' : '📁') : '📄';
        var nodeHtml =
            '<div class="webshell-tree-node" data-depth="' + depth + '">' +
            '<div class="webshell-tree-row' + (isActive ? ' active' : '') + '">' +
            '<button type="button" class="webshell-tree-toggle' + (canExpand ? '' : ' empty') + '" data-path="' + escapeHtml(path) + '">' + (canExpand ? (isExpanded ? '▾' : '▸') : '·') + '</button>' +
            '<button type="button" class="webshell-dir-item' + (isDir ? ' is-dir' : ' is-file') + '" data-path="' + escapeHtml(path) + '" data-isdir="' + (isDir ? '1' : '0') + '"><span class="webshell-tree-icon">' + icon + '</span><span class="webshell-tree-name">' + escapeHtml(name) + '</span></button>' +
            '</div>';
        if (isDir && hasChildren && isExpanded) {
            nodeHtml += '<div class="webshell-tree-children">';
            for (var j = 0; j < children.length; j++) {
                nodeHtml += renderNode(children[j], depth + 1);
            }
            nodeHtml += '</div>';
        }
        nodeHtml += '</div>';
        return nodeHtml;
    }

    treeEl.innerHTML = '<div class="webshell-tree-root">' + renderNode({ path: '.', name: '/', isDir: true }, 0) + '</div>';
    treeEl.querySelectorAll('.webshell-tree-toggle').forEach(function (btn) {
        btn.addEventListener('click', function (e) {
            e.preventDefault();
            e.stopPropagation();
            var p = normalizeWebshellPath(btn.getAttribute('data-path') || '.');
            if (expanded[p] !== false) {
                expanded[p] = false;
                renderDirectoryTree(curr, items, conn || webshellCurrentConn);
                return;
            }
            if (loaded[p] === true) {
                expanded[p] = true;
                renderDirectoryTree(curr, items, conn || webshellCurrentConn);
                return;
            }
            fetchWebshellDirectoryItems(conn || webshellCurrentConn, p).then(function (subItems) {
                var nextChildren = (subItems || []).map(function (it) {
                    return {
                        path: p === '.' ? normalizeWebshellPath(it.name) : normalizeWebshellPath(p + '/' + it.name),
                        name: it.name,
                        isDir: !!it.isDir
                    };
                }).filter(function (n) { return !!n.path; }).sort(function (a, b) {
                    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
                    return (a.name || '').localeCompare(b.name || '');
                });
                tree[p] = nextChildren;
                nextChildren.forEach(function (childNode) {
                    if (childNode.isDir) {
                        if (!tree[childNode.path]) tree[childNode.path] = [];
                        if (loaded[childNode.path] == null) loaded[childNode.path] = false;
                    }
                });
                loaded[p] = true;
                expanded[p] = true;
                renderDirectoryTree(curr, items, conn || webshellCurrentConn);
            });
        });
    });
    treeEl.querySelectorAll('.webshell-dir-item').forEach(function (btn) {
        btn.addEventListener('click', function () {
            var p = normalizeWebshellPath(btn.getAttribute('data-path') || '.');
            var isDir = btn.getAttribute('data-isdir') === '1';
            var pathInput = document.getElementById('webshell-file-path');
            if (isDir) {
                if (pathInput) pathInput.value = p;
                webshellFileListDir(webshellCurrentConn, p);
                return;
            }
            var listEl = document.getElementById('webshell-file-list');
            var browsePath = p.replace(/\/[^/]+$/, '') || '.';
            if (listEl) webshellFileRead(webshellCurrentConn, p, listEl, browsePath);
        });
    });
}

function webshellFileListApplyFilter() {
    var listEl = document.getElementById('webshell-file-list');
    var path = listEl && listEl.dataset.currentPath ? listEl.dataset.currentPath : (document.getElementById('webshell-file-path') && document.getElementById('webshell-file-path').value.trim()) || '.';
    var raw = listEl && listEl.dataset.rawOutput ? listEl.dataset.rawOutput : '';
    var filterInput = document.getElementById('webshell-file-filter');
    var filter = filterInput ? filterInput.value : '';
    if (!listEl || !raw) return;
    renderFileList(listEl, path, raw, webshellCurrentConn, filter);
}

function webshellFileMkdir(conn, pathInput) {
    if (!conn || typeof apiFetch === 'undefined') return;
    var base = (pathInput && pathInput.value.trim()) || '.';
    var name = prompt(wsT('webshell.newDir') || '', 'newdir');
    if (name == null || !name.trim()) return;
    var path = base === '.' ? name.trim() : base + '/' + name.trim();
    apiFetch('/api/webshell/file', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ url: conn.url, password: conn.password || '', type: conn.type || 'php', method: (conn.method || 'post').toLowerCase(), cmd_param: conn.cmdParam || '', action: 'mkdir', path: path }) })
        .then(function (r) { return r.json(); })
        .then(function () { webshellFileListDir(conn, base); })
        .catch(function () { webshellFileListDir(conn, base); });
}

function webshellFileNewFile(conn, pathInput) {
    if (!conn || typeof apiFetch === 'undefined') return;
    var base = (pathInput && pathInput.value.trim()) || '.';
    var name = prompt(wsT('webshell.newFile') || '', 'newfile.txt');
    if (name == null || !name.trim()) return;
    var path = base === '.' ? name.trim() : base + '/' + name.trim();
    var content = prompt('（）', '');
    if (content === null) return;
    var listEl = document.getElementById('webshell-file-list');
    webshellFileWrite(conn, path, content || '', function () { webshellFileListDir(conn, base); }, listEl);
}

function webshellFileUpload(conn, pathInput) {
    if (!conn || typeof apiFetch === 'undefined') return;
    var base = (pathInput && pathInput.value.trim()) || '.';
    var input = document.createElement('input');
    input.type = 'file';
    input.multiple = false;
    input.onchange = function () {
        var file = input.files && input.files[0];
        if (!file) return;
        var reader = new FileReader();
        reader.onload = function () {
            var buf = reader.result;
            var bin = new Uint8Array(buf);
            var CHUNK = 32000;
            var base64Chunks = [];
            for (var i = 0; i < bin.length; i += CHUNK) {
                var slice = bin.subarray(i, Math.min(i + CHUNK, bin.length));
                var b64 = btoa(String.fromCharCode.apply(null, slice));
                base64Chunks.push(b64);
            }
            var path = base === '.' ? file.name : base + '/' + file.name;
            var listEl = document.getElementById('webshell-file-list');
            if (listEl) listEl.innerHTML = '<div class="webshell-loading">' + (wsT('webshell.upload') || '') + '...</div>';
            var idx = 0;
            function sendNext() {
                if (idx >= base64Chunks.length) {
                    webshellFileListDir(conn, base);
                    return;
                }
                apiFetch('/api/webshell/file', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ url: conn.url, password: conn.password || '', type: conn.type || 'php', method: (conn.method || 'post').toLowerCase(), cmd_param: conn.cmdParam || '', action: 'upload_chunk', path: path, content: base64Chunks[idx], chunk_index: idx }) })
                    .then(function (r) { return r.json(); })
                    .then(function () { idx++; sendNext(); })
                    .catch(function () { idx++; sendNext(); });
            }
            sendNext();
        };
        reader.readAsArrayBuffer(file);
    };
    input.click();
}

function webshellFileRename(conn, oldPath, oldName, listEl) {
    if (!conn || typeof apiFetch === 'undefined') return;
    var newName = prompt((wsT('webshell.rename') || '') + ': ' + oldName, oldName);
    if (newName == null || newName.trim() === '') return;
    var parts = oldPath.split('/');
    var dir = parts.length > 1 ? parts.slice(0, -1).join('/') + '/' : '';
    var newPath = dir + newName.trim();
    apiFetch('/api/webshell/file', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ url: conn.url, password: conn.password || '', type: conn.type || 'php', method: (conn.method || 'post').toLowerCase(), cmd_param: conn.cmdParam || '', action: 'rename', path: oldPath, target_path: newPath }) })
        .then(function (r) { return r.json(); })
        .then(function () { webshellFileListDir(conn, document.getElementById('webshell-file-path').value.trim() || '.'); })
        .catch(function () { webshellFileListDir(conn, document.getElementById('webshell-file-path').value.trim() || '.'); });
}

function webshellBatchDelete(conn, pathInput) {
    if (!conn) return;
    var listEl = document.getElementById('webshell-file-list');
    var checked = listEl ? listEl.querySelectorAll('.webshell-file-cb:checked') : [];
    var paths = [];
    checked.forEach(function (cb) { paths.push(cb.getAttribute('data-path')); });
    if (paths.length === 0) { alert(wsT('webshell.batchDelete') + '：'); return; }
    if (!confirm(wsT('webshell.batchDelete') + '： ' + paths.length + ' ？')) return;
    var base = (pathInput && pathInput.value.trim()) || '.';
    var i = 0;
    function delNext() {
        if (i >= paths.length) { webshellFileListDir(conn, base); return; }
        webshellFileDelete(conn, paths[i], function () { i++; delNext(); });
    }
    delNext();
}

function webshellBatchDownload(conn, pathInput) {
    if (!conn) return;
    var listEl = document.getElementById('webshell-file-list');
    var checked = listEl ? listEl.querySelectorAll('.webshell-file-cb:checked') : [];
    var paths = [];
    checked.forEach(function (cb) { paths.push(cb.getAttribute('data-path')); });
    if (paths.length === 0) { alert(wsT('webshell.batchDownload') + '：'); return; }
    paths.forEach(function (path) { webshellFileDownload(conn, path); });
}

// English note.
function webshellFileDownload(conn, path) {
    if (typeof apiFetch === 'undefined') return;
    apiFetch('/api/webshell/file', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: conn.url, password: conn.password || '', type: conn.type || 'php', method: (conn.method || 'post').toLowerCase(), cmd_param: conn.cmdParam || '', action: 'read', path: path })
    }).then(function (r) { return r.json(); })
        .then(function (data) {
            var content = (data && data.output) != null ? data.output : (data.error || '');
            var name = path.replace(/^.*[/\\]/, '') || 'download.txt';
            var blob = new Blob([content], { type: 'application/octet-stream' });
            var a = document.createElement('a');
            a.href = URL.createObjectURL(blob);
            a.download = name;
            a.click();
            URL.revokeObjectURL(a.href);
        })
        .catch(function (err) { alert(wsT('webshell.execError') + ': ' + (err && err.message ? err.message : '')); });
}

function webshellFileRead(conn, path, listEl, browsePath) {
    if (typeof apiFetch === 'undefined') return;
    listEl.innerHTML = '<div class="webshell-loading">' + wsT('webshell.readFile') + '...</div>';
    apiFetch('/api/webshell/file', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: conn.url, password: conn.password || '', type: conn.type || 'php', method: (conn.method || 'post').toLowerCase(), cmd_param: conn.cmdParam || '', action: 'read', path: path })
    }).then(function (r) { return r.json(); })
        .then(function (data) {
            const out = (data && data.output) ? data.output : (data.error || '');
            var backPath = (browsePath && String(browsePath).trim()) ? String(browsePath).trim() : ((document.getElementById('webshell-file-path') && document.getElementById('webshell-file-path').value.trim()) || '.');
            if (backPath === path) {
                // English note.
                backPath = path.replace(/\/[^/]+$/, '') || '.';
            }
            listEl.innerHTML = '<div class="webshell-file-content"><pre>' + escapeHtml(out) + '</pre><button type="button" class="btn-ghost" id="webshell-file-back-btn" data-back-path="' + escapeHtml(backPath) + '">' + wsT('webshell.back') + '</button></div>';
            var backBtn = document.getElementById('webshell-file-back-btn');
            if (backBtn) {
                backBtn.addEventListener('click', function () {
                    var p = backBtn.getAttribute('data-back-path') || '.';
                    webshellFileListDir(webshellCurrentConn, p);
                });
            }
        })
        .catch(function (err) {
            listEl.innerHTML = '<div class="webshell-file-error">' + escapeHtml(err && err.message ? err.message : '') + '</div>';
        });
}

function webshellFileEdit(conn, path, listEl) {
    if (typeof apiFetch === 'undefined') return;
    listEl.innerHTML = '<div class="webshell-loading">' + wsT('webshell.editFile') + '...</div>';
    apiFetch('/api/webshell/file', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: conn.url, password: conn.password || '', type: conn.type || 'php', method: (conn.method || 'post').toLowerCase(), cmd_param: conn.cmdParam || '', action: 'read', path: path })
    }).then(function (r) { return r.json(); })
        .then(function (data) {
            const content = (data && data.output) ? data.output : (data.error || '');
            const pathInput = document.getElementById('webshell-file-path');
            const currentPath = pathInput ? pathInput.value.trim() || '.' : '.';
            listEl.innerHTML =
                '<div class="webshell-file-edit-wrap">' +
                '<div class="webshell-file-edit-path">' + escapeHtml(path) + '</div>' +
                '<textarea id="webshell-edit-textarea" class="webshell-file-edit-textarea" rows="18">' + escapeHtml(content) + '</textarea>' +
                '<div class="webshell-file-edit-actions">' +
                '<button type="button" class="btn-primary btn-sm" id="webshell-edit-save">' + wsT('webshell.saveFile') + '</button> ' +
                '<button type="button" class="btn-ghost btn-sm" id="webshell-edit-cancel">' + wsT('webshell.cancelEdit') + '</button>' +
                '</div></div>';
            document.getElementById('webshell-edit-save').addEventListener('click', function () {
                const textarea = document.getElementById('webshell-edit-textarea');
                const newContent = textarea ? textarea.value : '';
                webshellFileWrite(webshellCurrentConn, path, newContent, function () {
                    webshellFileListDir(webshellCurrentConn, currentPath);
                }, listEl);
            });
            document.getElementById('webshell-edit-cancel').addEventListener('click', function () {
                webshellFileListDir(webshellCurrentConn, currentPath);
            });
        })
        .catch(function (err) {
            listEl.innerHTML = '<div class="webshell-file-error">' + escapeHtml(err && err.message ? err.message : '') + '</div>';
        });
}

function webshellFileWrite(conn, path, content, onDone, listEl) {
    if (typeof apiFetch === 'undefined') return;
    if (listEl) listEl.innerHTML = '<div class="webshell-loading">' + wsT('webshell.saveFile') + '...</div>';
    apiFetch('/api/webshell/file', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: conn.url, password: conn.password || '', type: conn.type || 'php', method: (conn.method || 'post').toLowerCase(), cmd_param: conn.cmdParam || '', action: 'write', path: path, content: content })
    }).then(function (r) { return r.json(); })
        .then(function (data) {
            if (data && !data.ok && data.error && listEl) {
                listEl.innerHTML = '<div class="webshell-file-error">' + escapeHtml(data.error) + '</div><pre class="webshell-file-raw">' + escapeHtml(data.output || '') + '</pre>';
                return;
            }
            if (onDone) onDone();
        })
        .catch(function (err) {
            if (listEl) listEl.innerHTML = '<div class="webshell-file-error">' + escapeHtml(err && err.message ? err.message : wsT('webshell.execError')) + '</div>';
        });
}

function webshellFileDelete(conn, path, onDone) {
    if (typeof apiFetch === 'undefined') return;
    apiFetch('/api/webshell/file', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url: conn.url, password: conn.password || '', type: conn.type || 'php', method: (conn.method || 'post').toLowerCase(), cmd_param: conn.cmdParam || '', action: 'delete', path: path })
    }).then(function (r) { return r.json(); })
        .then(function () { if (onDone) onDone(); })
        .catch(function () { if (onDone) onDone(); });
}

// English note.
function deleteWebshell(id) {
    if (!confirm(wsT('webshell.deleteConfirm'))) return;
    if (currentWebshellId === id) destroyWebshellTerminal();
    if (currentWebshellId === id) currentWebshellId = null;
    // English note.
    delete webshellPersistLoadedByConn[id];
    if (webshellPersistSaveTimersByConn[id]) {
        clearTimeout(webshellPersistSaveTimersByConn[id]);
        delete webshellPersistSaveTimersByConn[id];
    }
    delete webshellTerminalSessionsByConn[id];
    var dbStateKey = getWebshellDbStateStorageKey({ id: id });
    if (dbStateKey) delete webshellDbConfigByConn[dbStateKey];
    Object.keys(webshellTerminalLogsByConn).forEach(function (k) {
        if (k === id || k.indexOf(id + '::') === 0) delete webshellTerminalLogsByConn[k];
    });
    Object.keys(webshellHistoryByConn).forEach(function (k) {
        if (k === id || k.indexOf(id + '::') === 0) delete webshellHistoryByConn[k];
    });
    if (typeof apiFetch === 'undefined') return;
    apiFetch('/api/webshell/connections/' + encodeURIComponent(id), { method: 'DELETE' })
        .then(function () {
            return refreshWebshellConnectionsFromServer();
        })
        .then(function () {
            const workspace = document.getElementById('webshell-workspace');
            if (workspace) {
                workspace.innerHTML = '<div class="webshell-workspace-placeholder">' + wsT('webshell.selectOrAdd') + '</div>';
            }
        })
        .catch(function (e) {
            console.warn(' WebShell ', e);
            refreshWebshellConnectionsFromServer();
        });
}

// English note.
function showAddWebshellModal() {
    var editIdEl = document.getElementById('webshell-edit-id');
    if (editIdEl) editIdEl.value = '';
    document.getElementById('webshell-url').value = '';
    document.getElementById('webshell-password').value = '';
    document.getElementById('webshell-type').value = 'php';
    document.getElementById('webshell-method').value = 'post';
    document.getElementById('webshell-cmd-param').value = '';
    document.getElementById('webshell-remark').value = '';
    var titleEl = document.getElementById('webshell-modal-title');
    if (titleEl) titleEl.textContent = wsT('webshell.addConnection');
    var modal = document.getElementById('webshell-modal');
    if (modal) modal.style.display = 'block';
}

// English note.
function showEditWebshellModal(connId) {
    var conn = webshellConnections.find(function (c) { return c.id === connId; });
    if (!conn) return;
    var editIdEl = document.getElementById('webshell-edit-id');
    if (editIdEl) editIdEl.value = conn.id;
    document.getElementById('webshell-url').value = conn.url || '';
    document.getElementById('webshell-password').value = conn.password || '';
    document.getElementById('webshell-type').value = conn.type || 'php';
    document.getElementById('webshell-method').value = (conn.method || 'post').toLowerCase();
    document.getElementById('webshell-cmd-param').value = conn.cmdParam || '';
    document.getElementById('webshell-remark').value = conn.remark || '';
    var titleEl = document.getElementById('webshell-modal-title');
    if (titleEl) titleEl.textContent = wsT('webshell.editConnectionTitle');
    var modal = document.getElementById('webshell-modal');
    if (modal) modal.style.display = 'block';
}

// English note.
function closeWebshellModal() {
    var editIdEl = document.getElementById('webshell-edit-id');
    if (editIdEl) editIdEl.value = '';
    var modal = document.getElementById('webshell-modal');
    if (modal) modal.style.display = 'none';
}

// English note.
function refreshWebshellUIOnLanguageChange() {
    var page = typeof window.currentPage === 'function' ? window.currentPage() : (window.currentPage || '');
    if (page !== 'webshell') return;

    renderWebshellList();
    var workspace = document.getElementById('webshell-workspace');
    if (workspace) {
        if (!currentWebshellId || !webshellCurrentConn) {
            workspace.innerHTML = '<div class="webshell-workspace-placeholder" data-i18n="webshell.selectOrAdd">' + wsT('webshell.selectOrAdd') + '</div>';
        } else {
            // English note.
            var tabTerminal = workspace.querySelector('.webshell-tab[data-tab="terminal"]');
            var tabFile = workspace.querySelector('.webshell-tab[data-tab="file"]');
            var tabAi = workspace.querySelector('.webshell-tab[data-tab="ai"]');
            var tabDb = workspace.querySelector('.webshell-tab[data-tab="db"]');
            var tabMemo = workspace.querySelector('.webshell-tab[data-tab="memo"]');
            if (tabTerminal) tabTerminal.textContent = wsT('webshell.tabTerminal');
            if (tabFile) tabFile.textContent = wsT('webshell.tabFileManager');
            if (tabAi) tabAi.textContent = wsT('webshell.tabAiAssistant') || 'AI ';
            if (tabDb) tabDb.textContent = wsT('webshell.tabDbManager') || '';
            if (tabMemo) tabMemo.textContent = wsT('webshell.tabMemo') || '';

            var quickLabel = workspace.querySelector('.webshell-quick-label');
            if (quickLabel) quickLabel.textContent = (wsT('webshell.quickCommands') || '') + ':';
            var terminalClearBtn = document.getElementById('webshell-terminal-clear');
            if (terminalClearBtn) {
                terminalClearBtn.title = wsT('webshell.clearScreen') || '';
                terminalClearBtn.textContent = wsT('webshell.clearScreen') || '';
            }
            var terminalCopyBtn = document.getElementById('webshell-terminal-copy-log');
            if (terminalCopyBtn) {
                terminalCopyBtn.title = wsT('webshell.copyTerminalLog') || '';
                terminalCopyBtn.textContent = wsT('webshell.copyTerminalLog') || '';
            }
            setWebshellTerminalStatus(webshellTerminalRunning);
            if (webshellCurrentConn) renderWebshellTerminalSessions(webshellCurrentConn);
            var pathLabel = workspace.querySelector('.webshell-file-toolbar label span');
            var fileSidebarTitle = workspace.querySelector('.webshell-file-sidebar-title');
            var fileMoreActionsBtn = workspace.querySelector('.webshell-toolbar-actions-btn');
            var listDirBtn = document.getElementById('webshell-list-dir');
            var parentDirBtn = document.getElementById('webshell-parent-dir');
            if (pathLabel) pathLabel.textContent = wsT('webshell.filePath');
            if (fileSidebarTitle) fileSidebarTitle.textContent = wsT('webshell.dirTree') || '';
            if (fileMoreActionsBtn) fileMoreActionsBtn.textContent = wsT('webshell.moreActions') || '';
            if (listDirBtn) listDirBtn.textContent = wsT('webshell.listDir');
            if (parentDirBtn) parentDirBtn.textContent = wsT('webshell.parentDir');
            // English note.
            var refreshBtn = document.getElementById('webshell-file-refresh');
            var mkdirBtn = document.getElementById('webshell-mkdir-btn');
            var newFileBtn = document.getElementById('webshell-newfile-btn');
            var uploadBtn = document.getElementById('webshell-upload-btn');
            var batchDeleteBtn = document.getElementById('webshell-batch-delete-btn');
            var batchDownloadBtn = document.getElementById('webshell-batch-download-btn');
            var filterInput = document.getElementById('webshell-file-filter');
            if (refreshBtn) { refreshBtn.title = wsT('webshell.refresh') || ''; refreshBtn.textContent = wsT('webshell.refresh') || ''; }
            if (mkdirBtn) mkdirBtn.textContent = wsT('webshell.newDir') || '';
            if (newFileBtn) newFileBtn.textContent = wsT('webshell.newFile') || '';
            if (uploadBtn) uploadBtn.textContent = wsT('webshell.upload') || '';
            if (batchDeleteBtn) batchDeleteBtn.textContent = wsT('webshell.batchDelete') || '';
            if (batchDownloadBtn) batchDownloadBtn.textContent = wsT('webshell.batchDownload') || '';
            if (filterInput) filterInput.placeholder = wsT('webshell.filterPlaceholder') || '';

            // English note.
            var aiNewConvBtn = document.getElementById('webshell-ai-new-conv');
            if (aiNewConvBtn) aiNewConvBtn.textContent = wsT('webshell.aiNewConversation') || '';
            var aiInput = document.getElementById('webshell-ai-input');
            if (aiInput) aiInput.placeholder = wsT('webshell.aiPlaceholder') || '：';
            var aiSendBtn = document.getElementById('webshell-ai-send');
            if (aiSendBtn) aiSendBtn.textContent = wsT('webshell.aiSend') || '';
            var aiMemoTitle = document.querySelector('.webshell-memo-head span');
            if (aiMemoTitle) aiMemoTitle.textContent = wsT('webshell.aiMemo') || '';
            var aiMemoClearBtn = document.getElementById('webshell-ai-memo-clear');
            if (aiMemoClearBtn) aiMemoClearBtn.textContent = wsT('webshell.aiMemoClear') || '';
            var aiMemoInput = document.getElementById('webshell-ai-memo-input');
            if (aiMemoInput) aiMemoInput.placeholder = wsT('webshell.aiMemoPlaceholder') || '、、...';
            var aiMemoStatus = document.getElementById('webshell-ai-memo-status');
            if (aiMemoStatus && !aiMemoStatus.classList.contains('error')) {
                var savingText = wsT('webshell.aiMemoSaving') || '...';
                var savedText = wsT('webshell.aiMemoSaved') || '';
                aiMemoStatus.textContent = aiMemoStatus.textContent === savingText ? savingText : savedText;
            }
            var dbTypeLabel = document.querySelector('#webshell-db-type') ? document.querySelector('#webshell-db-type').closest('label') : null;
            if (dbTypeLabel && dbTypeLabel.querySelector('span')) dbTypeLabel.querySelector('span').textContent = wsT('webshell.dbType') || '';
            var dbProfileNameLabel = document.querySelector('#webshell-db-profile-name') ? document.querySelector('#webshell-db-profile-name').closest('label') : null;
            if (dbProfileNameLabel && dbProfileNameLabel.querySelector('span')) dbProfileNameLabel.querySelector('span').textContent = wsT('webshell.dbProfileName') || '';
            var dbHostLabel = document.querySelector('#webshell-db-host') ? document.querySelector('#webshell-db-host').closest('label') : null;
            if (dbHostLabel && dbHostLabel.querySelector('span')) dbHostLabel.querySelector('span').textContent = wsT('webshell.dbHost') || '';
            var dbPortLabel = document.querySelector('#webshell-db-port') ? document.querySelector('#webshell-db-port').closest('label') : null;
            if (dbPortLabel && dbPortLabel.querySelector('span')) dbPortLabel.querySelector('span').textContent = wsT('webshell.dbPort') || '';
            var dbUserLabel = document.querySelector('#webshell-db-user') ? document.querySelector('#webshell-db-user').closest('label') : null;
            if (dbUserLabel && dbUserLabel.querySelector('span')) dbUserLabel.querySelector('span').textContent = wsT('webshell.dbUsername') || '';
            var dbPassLabel = document.querySelector('#webshell-db-pass') ? document.querySelector('#webshell-db-pass').closest('label') : null;
            if (dbPassLabel && dbPassLabel.querySelector('span')) dbPassLabel.querySelector('span').textContent = wsT('webshell.dbPassword') || '';
            var dbNameLabel = document.querySelector('#webshell-db-name') ? document.querySelector('#webshell-db-name').closest('label') : null;
            if (dbNameLabel && dbNameLabel.querySelector('span')) dbNameLabel.querySelector('span').textContent = wsT('webshell.dbName') || '';
            var dbSqliteLabel = document.querySelector('#webshell-db-sqlite-path') ? document.querySelector('#webshell-db-sqlite-path').closest('label') : null;
            if (dbSqliteLabel && dbSqliteLabel.querySelector('span')) dbSqliteLabel.querySelector('span').textContent = wsT('webshell.dbSqlitePath') || 'SQLite ';
            var dbSchemaTitle = document.querySelector('.webshell-db-sidebar-head span');
            if (dbSchemaTitle) dbSchemaTitle.textContent = wsT('webshell.dbSchema') || '';
            var dbLoadSchemaBtn = document.getElementById('webshell-db-load-schema-btn');
            if (dbLoadSchemaBtn) dbLoadSchemaBtn.textContent = wsT('webshell.dbLoadSchema') || '';
            var dbTemplateBtn = document.getElementById('webshell-db-template-btn');
            if (dbTemplateBtn) dbTemplateBtn.textContent = wsT('webshell.dbTemplateSql') || ' SQL';
            var dbClearBtn = document.getElementById('webshell-db-clear-btn');
            if (dbClearBtn) dbClearBtn.textContent = wsT('webshell.dbClearSql') || ' SQL';
            var dbRunBtn = document.getElementById('webshell-db-run-btn');
            if (dbRunBtn) dbRunBtn.textContent = wsT('webshell.dbRunSql') || ' SQL';
            var dbTestBtn = document.getElementById('webshell-db-test-btn');
            if (dbTestBtn) dbTestBtn.textContent = wsT('webshell.dbTest') || '';
            var dbSql = document.getElementById('webshell-db-sql');
            if (dbSql) dbSql.placeholder = wsT('webshell.dbSqlPlaceholder') || ' SQL，：SELECT version();';
            var dbTitle = document.querySelector('.webshell-db-output-title');
            if (dbTitle) dbTitle.textContent = wsT('webshell.dbOutput') || '';
            var dbHint = document.querySelector('.webshell-db-hint');
            if (dbHint) dbHint.textContent = wsT('webshell.dbCliHint') || '，（mysql/psql/sqlite3/sqlcmd）';
            var dbTreeHint = document.querySelector('.webshell-db-sidebar-hint');
            if (dbTreeHint) dbTreeHint.textContent = wsT('webshell.dbSelectTableHint') || ' SQL';
            var dbAddProfileBtn = document.getElementById('webshell-db-add-profile-btn');
            if (dbAddProfileBtn) dbAddProfileBtn.textContent = '+ ' + (wsT('webshell.dbAddProfile') || '');
            var dbProfileModalTitle = document.getElementById('webshell-db-profile-modal-title');
            if (dbProfileModalTitle) dbProfileModalTitle.textContent = wsT('webshell.editConnectionTitle') || '';
            var dbProfileCancelBtn = document.getElementById('webshell-db-profile-cancel-btn');
            if (dbProfileCancelBtn) dbProfileCancelBtn.textContent = '';
            var dbProfileSaveBtn = document.getElementById('webshell-db-profile-save-btn');
            if (dbProfileSaveBtn) dbProfileSaveBtn.textContent = '';
            document.querySelectorAll('.webshell-db-profile-menu[data-action="edit"]').forEach(function (el) {
                el.title = wsT('webshell.editConnection') || '';
            });
            document.querySelectorAll('.webshell-db-profile-menu[data-action="delete"]').forEach(function (el) {
                el.title = wsT('webshell.dbDeleteProfile') || '';
            });
            var dbTree = document.getElementById('webshell-db-schema-tree');
            if (dbTree && !dbTree.querySelector('.webshell-db-group')) {
                dbTree.innerHTML = '<div class="webshell-empty">' + escapeHtml(wsT('webshell.dbNoSchema') || '，') + '</div>';
            }

            // English note.
            var aiMessages = document.getElementById('webshell-ai-messages');
            if (aiMessages) {
                var hasUserMsg = !!aiMessages.querySelector('.webshell-ai-msg.user');
                var msgNodes = aiMessages.querySelectorAll('.webshell-ai-msg');
                if (!hasUserMsg && msgNodes.length <= 1) {
                    var readyMsg = wsT('webshell.aiSystemReadyMessage') || '。，。';
                    aiMessages.innerHTML = '';
                    var readyDiv = document.createElement('div');
                    readyDiv.className = 'webshell-ai-msg assistant';
                    readyDiv.textContent = readyMsg;
                    aiMessages.appendChild(readyDiv);
                }
            }

            var pathInput = document.getElementById('webshell-file-path');
            var fileListEl = document.getElementById('webshell-file-list');
            if (fileListEl && webshellCurrentConn && pathInput) {
                webshellFileListDir(webshellCurrentConn, pathInput.value.trim() || '.');
            }

            // English note.
            var connSearchEl = document.getElementById('webshell-conn-search');
            if (connSearchEl) {
                var ph = wsT('webshell.searchPlaceholder') || '...';
                connSearchEl.setAttribute('placeholder', ph);
                connSearchEl.placeholder = ph;
            }
        }
    }

    var modal = document.getElementById('webshell-modal');
    if (modal && modal.style.display === 'block') {
        var titleEl = document.getElementById('webshell-modal-title');
        var editIdEl = document.getElementById('webshell-edit-id');
        if (titleEl) {
            titleEl.textContent = (editIdEl && editIdEl.value) ? wsT('webshell.editConnectionTitle') : wsT('webshell.addConnection');
        }
        if (typeof window.applyTranslations === 'function') {
            window.applyTranslations(modal);
        }
    }
}

document.addEventListener('languagechange', function () {
    refreshWebshellUIOnLanguageChange();
});

// English note.
document.addEventListener('conversation-deleted', function (e) {
    var id = e.detail && e.detail.conversationId;
    if (!id || !currentWebshellId || !webshellCurrentConn) return;
    var listEl = document.getElementById('webshell-ai-conv-list');
    if (listEl) fetchAndRenderWebshellAiConvList(webshellCurrentConn, listEl);
    if (webshellAiConvMap[webshellCurrentConn.id] === id) {
        delete webshellAiConvMap[webshellCurrentConn.id];
        var msgs = document.getElementById('webshell-ai-messages');
        if (msgs) msgs.innerHTML = '';
    }
});

// English note.
function testWebshellConnection() {
    var url = (document.getElementById('webshell-url') || {}).value;
    if (url && typeof url.trim === 'function') url = url.trim();
    if (!url) {
        alert(wsT('webshell.url') ? (wsT('webshell.url') + ' ') : ' Shell ');
        return;
    }
    var password = (document.getElementById('webshell-password') || {}).value;
    if (password && typeof password.trim === 'function') password = password.trim(); else password = '';
    var type = (document.getElementById('webshell-type') || {}).value || 'php';
    var method = ((document.getElementById('webshell-method') || {}).value || 'post').toLowerCase();
    var cmdParam = (document.getElementById('webshell-cmd-param') || {}).value;
    if (cmdParam && typeof cmdParam.trim === 'function') cmdParam = cmdParam.trim(); else cmdParam = '';
    var btn = document.getElementById('webshell-test-btn');
    if (btn) { btn.disabled = true; btn.textContent = (typeof wsT === 'function' ? wsT('common.refresh') : '') + '...'; }
    if (typeof apiFetch === 'undefined') {
        if (btn) { btn.disabled = false; btn.textContent = wsT('webshell.testConnectivity'); }
        alert(wsT('webshell.testFailed') || '');
        return;
    }
    apiFetch('/api/webshell/exec', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            url: url,
            password: password || '',
            type: type,
            method: method === 'get' ? 'get' : 'post',
            cmd_param: cmdParam || '',
            command: 'echo 1'
        })
    })
        .then(function (r) { return r.json(); })
        .then(function (data) {
            if (btn) { btn.disabled = false; btn.textContent = wsT('webshell.testConnectivity'); }
            if (!data) {
                alert(wsT('webshell.testFailed') || '');
                return;
            }
            // English note.
            var output = (data.output != null) ? String(data.output).trim() : '';
            var reallyOk = data.ok && output === '1';
            if (reallyOk) {
                alert(wsT('webshell.testSuccess') || '，Shell ');
            } else {
                var msg;
                if (data.ok && output !== '1')
                    msg = wsT('webshell.testNoExpectedOutput') || 'Shell ，';
                else
                    msg = (data.error) ? data.error : (wsT('webshell.testFailed') || '');
                if (data.http_code) msg += ' (HTTP ' + data.http_code + ')';
                alert(msg);
            }
        })
        .catch(function (e) {
            if (btn) { btn.disabled = false; btn.textContent = wsT('webshell.testConnectivity'); }
            alert((wsT('webshell.testFailed') || '') + ': ' + (e && e.message ? e.message : String(e)));
        });
}

// English note.
function saveWebshellConnection() {
    var url = (document.getElementById('webshell-url') || {}).value;
    if (url && typeof url.trim === 'function') url = url.trim();
    if (!url) {
        alert(' Shell ');
        return;
    }
    var password = (document.getElementById('webshell-password') || {}).value;
    if (password && typeof password.trim === 'function') password = password.trim(); else password = '';
    var type = (document.getElementById('webshell-type') || {}).value || 'php';
    var method = ((document.getElementById('webshell-method') || {}).value || 'post').toLowerCase();
    var cmdParam = (document.getElementById('webshell-cmd-param') || {}).value;
    if (cmdParam && typeof cmdParam.trim === 'function') cmdParam = cmdParam.trim(); else cmdParam = '';
    var remark = (document.getElementById('webshell-remark') || {}).value;
    if (remark && typeof remark.trim === 'function') remark = remark.trim(); else remark = '';

    var editIdEl = document.getElementById('webshell-edit-id');
    var editId = editIdEl ? editIdEl.value.trim() : '';
    var body = { url: url, password: password, type: type, method: method === 'get' ? 'get' : 'post', cmd_param: cmdParam, remark: remark || url };
    if (typeof apiFetch === 'undefined') return;

    var reqUrl = editId ? ('/api/webshell/connections/' + encodeURIComponent(editId)) : '/api/webshell/connections';
    var reqMethod = editId ? 'PUT' : 'POST';
    apiFetch(reqUrl, {
        method: reqMethod,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
    })
        .then(function (r) { return r.json(); })
        .then(function () {
            closeWebshellModal();
            return refreshWebshellConnectionsFromServer();
        })
        .then(function (list) {
            // English note.
            if (editId && currentWebshellId === editId && Array.isArray(list)) {
                var updated = list.find(function (c) { return c.id === editId; });
                if (updated) webshellCurrentConn = updated;
            }
        })
        .catch(function (e) {
            console.warn(' WebShell ', e);
            alert(e && e.message ? e.message : '');
        });
}
