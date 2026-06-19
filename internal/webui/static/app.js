'use strict';

const fields = {
  listenHost: document.querySelector('#listenHost'),
  listenPort: document.querySelector('#listenPort'),
  whitelist: document.querySelector('#whitelist'),
  whitelistPath: document.querySelector('#whitelistPath'),
  relayEnabled: document.querySelector('#relayEnabled'),
  relayRows: document.querySelector('#relayRows'),
  addRelayBtn: document.querySelector('#addRelayBtn'),
  statusText: document.querySelector('#statusText'),
  toggleBtn: document.querySelector('#toggleBtn'),
  saveBtn: document.querySelector('#saveBtn')
};

let currentState;
const proxyRows = [...document.querySelectorAll('.proxyRow')];

async function api(method, url, body) {
  const response = await fetch(url, {
    method,
    headers: body ? { 'content-type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined
  });
  const data = await response.json();
  if (!response.ok) throw new Error(data.error || response.statusText);
  return data;
}

function readForm() {
  return {
    config: {
      enabled: currentState.config.enabled,
      listen: {
        host: fields.listenHost.value.trim(),
        port: Number(fields.listenPort.value)
      },
      controlListen: currentState.config.controlListen,
      whitelistFile: currentState.config.whitelistFile || 'whitelist.txt',
      upstreams: readUpstreams(),
      relay: readRelay()
    },
    whitelist: fields.whitelist.value
  };
}

function readUpstreams() {
  const upstreams = {};
  for (const row of proxyRows) {
    const key = row.dataset.proxy;
    upstreams[key] = {
      enabled: row.querySelector('.proxyEnabled').checked,
      host: row.querySelector('.proxyHost').value.trim(),
      port: Number(row.querySelector('.proxyPort').value),
      username: row.querySelector('.proxyUsername').value,
      password: row.querySelector('.proxyPassword').value
    };
  }
  return upstreams;
}

function readRelay() {
  const rules = [...fields.relayRows.querySelectorAll('.relayRow')].map((row) => ({
    enabled: row.querySelector('.relayRuleEnabled').checked,
    entry: {
      host: row.querySelector('.relayEntryHost').value.trim(),
      port: Number(row.querySelector('.relayEntryPort').value)
    },
    exit: {
      host: row.querySelector('.relayExitHost').value.trim(),
      port: Number(row.querySelector('.relayExitPort').value)
    }
  }));
  return {
    enabled: fields.relayEnabled.checked,
    rules
  };
}

function render(state) {
  currentState = state;
  document.body.classList.toggle('running', state.running);
  document.body.classList.toggle('stopped', !state.running);
  fields.listenHost.value = state.config.listen.host;
  fields.listenPort.value = state.config.listen.port;
  renderUpstreams(state.config.upstreams);
  renderRelay(state.config.relay);
  fields.whitelist.value = state.whitelist || '';
  fields.whitelistPath.textContent = state.whitelistPath;
  fields.toggleBtn.textContent = state.running ? '停止' : '启动';
  const relayText = state.relayAddresses && state.relayAddresses.length
    ? ` / 中转 ${state.relayAddresses.join(', ')}`
    : '';
  fields.statusText.textContent = state.running ? `已启动 ${state.address}${relayText}` : '未启动';
}

function renderUpstreams(upstreams) {
  for (const row of proxyRows) {
    const key = row.dataset.proxy;
    const value = upstreams[key] || {};
    row.querySelector('.proxyEnabled').checked = Boolean(value.enabled);
    row.querySelector('.proxyHost').value = value.host || '';
    row.querySelector('.proxyPort').value = value.port || '';
    row.querySelector('.proxyUsername').value = value.username || '';
    row.querySelector('.proxyPassword').value = value.password || '';
  }
}

function renderRelay(relay) {
  fields.relayEnabled.checked = Boolean(relay?.enabled);
  fields.relayRows.innerHTML = '';
  const rules = relay?.rules?.length ? relay.rules : [defaultRelayRule()];
  for (const rule of rules) {
    fields.relayRows.appendChild(createRelayRow(rule));
  }
}

function defaultRelayRule() {
  return {
    enabled: true,
    entry: { host: '0.0.0.0', port: 34567 },
    exit: { host: '127.0.0.1', port: 45678 }
  };
}

function createRelayRow(rule) {
  const row = document.createElement('div');
  row.className = 'relayRow';
  row.innerHTML = `
    <label class="relayEnableCell">
      <input class="relayRuleEnabled" type="checkbox">
    </label>
    <input class="relayEntryHost" autocomplete="off">
    <input class="relayEntryPort" type="number" min="1" max="65535">
    <input class="relayExitHost" autocomplete="off">
    <input class="relayExitPort" type="number" min="1" max="65535">
    <button type="button" class="removeRelayBtn">删除</button>
  `;
  row.querySelector('.relayRuleEnabled').checked = rule.enabled !== false;
  row.querySelector('.relayEntryHost').value = rule.entry?.host || '0.0.0.0';
  row.querySelector('.relayEntryPort').value = rule.entry?.port || 34567;
  row.querySelector('.relayExitHost').value = rule.exit?.host || '127.0.0.1';
  row.querySelector('.relayExitPort').value = rule.exit?.port || 45678;
  row.querySelector('.removeRelayBtn').addEventListener('click', () => {
    row.remove();
    if (!fields.relayRows.querySelector('.relayRow')) {
      fields.relayRows.appendChild(createRelayRow(defaultRelayRule()));
    }
  });
  return row;
}

async function save() {
  render(await api('POST', '/api/config', readForm()));
}

fields.saveBtn.addEventListener('click', save);
fields.addRelayBtn.addEventListener('click', () => {
  fields.relayRows.appendChild(createRelayRow(defaultRelayRule()));
});
fields.toggleBtn.addEventListener('click', async () => {
  if (currentState.running) {
    render(await api('POST', '/api/stop'));
  } else {
    await save();
    render(await api('POST', '/api/start'));
  }
});

api('GET', '/api/state').then(render).catch((error) => {
  fields.statusText.textContent = error.message;
});
