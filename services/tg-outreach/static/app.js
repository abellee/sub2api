'use strict';
const $ = id => document.getElementById(id);
const fields = ['telegram_api_id', 'telegram_api_hash', 'message', 'send_interval', 'keywords', 'search_interval', 'join_interval', 'search_limit', 'prompt', 'api_base', 'api_key', 'model'];
const numbers = new Set(['telegram_api_id', 'send_interval', 'search_interval', 'join_interval', 'search_limit']);
let loggedIn = false, polling = false, latest = null, noticeTimer;
let selectedGroups = new Set();

function notice(message, error = false) {
  $('notice').textContent = message;
  $('notice').classList.toggle('error', error);
  $('notice').hidden = false;
  clearTimeout(noticeTimer);
  noticeTimer = setTimeout(() => { $('notice').hidden = true; }, error ? 10000 : 4500);
}

function showLogin() {
  loggedIn = false;
  latest = null;
  $('workspace').hidden = true;
  $('loginPage').hidden = false;
}

async function api(path, method = 'GET', body) {
  const options = {method, headers: {}, credentials: 'same-origin'};
  if (body !== undefined) {
    options.headers['Content-Type'] = 'application/json';
    options.body = JSON.stringify(body);
  }
  const response = await fetch('/api' + path, options);
  let data;
  try { data = await response.json(); } catch { throw new Error('服务返回异常，请检查服务是否运行'); }
  if (!response.ok) {
    if (response.status === 401) showLogin();
    throw new Error(data.detail || '请求失败');
  }
  return data;
}

function modelOption(value) {
  if (!value || [...$('model').options].some(option => option.value === value)) return;
  const option = document.createElement('option');
  option.value = value;
  option.textContent = value;
  $('model').append(option);
}

function secretHints(data) {
  $('hashHint').textContent = data.telegram_api_hash_saved ? '已保存，留空保留原值' : '尚未保存';
  $('apiKeyHint').textContent = data.api_key_saved ? '已保存，留空保留原值' : '尚未保存';
}

async function loadSettings() {
  const data = await api('/settings');
  selectedGroups = new Set(data.selected_group_ids || []);
  modelOption(data.model);
  for (const field of fields) $(field).value = data[field] ?? '';
  secretHints(data);
}

async function save() {
  const body = {selected_group_ids: [...selectedGroups]};
  for (const field of fields) {
    const input = $(field);
    if (!input.checkValidity()) { input.reportValidity(); throw new Error('请检查配置格式'); }
    body[field] = numbers.has(field) ? Number(input.value || 0) : input.value;
  }
  await api('/settings', 'PUT', body);
  $('telegram_api_hash').value = '';
  $('api_key').value = '';
  secretHints(await api('/settings'));
  $('saveHint').textContent = '配置已保存 · ' + new Date().toLocaleTimeString();
}

function date(value) { return value ? new Date(value * 1000).toLocaleString() : '—'; }

function table(id, rows) {
  const fragment = document.createDocumentFragment();
  if (!rows.length) {
    const tr = document.createElement('tr'), td = document.createElement('td');
    td.colSpan = 4; td.className = 'empty'; td.textContent = '暂无记录'; tr.append(td); fragment.append(tr);
  }
  for (const row of rows) {
    const tr = document.createElement('tr');
    for (const cell of row) {
      const td = document.createElement('td');
      if (cell instanceof Node) td.append(cell); else td.textContent = cell ?? '—';
      tr.append(td);
    }
    fragment.append(tr);
  }
  $(id).replaceChildren(fragment);
}

function visibleGroups() {
  const filter = $('groupFilter').value.toLocaleLowerCase();
  return (latest?.groups || []).filter(group => (group.title + ' ' + group.username).toLocaleLowerCase().includes(filter));
}

function renderGroups() {
  table('groupsTable', visibleGroups().map(group => {
    const checkbox = document.createElement('input');
    checkbox.type = 'checkbox'; checkbox.className = 'group-check';
    checkbox.checked = selectedGroups.has(group.id);
    checkbox.setAttribute('aria-label', '选择群聊 ' + group.title);
    checkbox.addEventListener('change', () => {
      if (checkbox.checked) selectedGroups.add(group.id); else selectedGroups.delete(group.id);
      updateSelectionCount();
    });
    return [checkbox, group.title, group.username ? '@' + group.username : '非公开群', group.id];
  }));
  updateSelectionCount();
}

function updateSelectionCount() {
  const available = (latest?.groups || []).filter(group => selectedGroups.has(group.id)).length;
  $('selectionCount').textContent = '已选 ' + available + ' 个群 · 修改后请保存';
}

function render(data) {
  latest = data;
  const authorized = data.telegram.authorized;
  $('accountBadge').textContent = authorized ? (data.telegram.name || 'Telegram 已连接') : '未登录 Telegram';
  $('accountBadge').classList.toggle('connected', authorized);
  $('telegramLogin').hidden = authorized;
  $('telegramConnected').hidden = !authorized;
  $('connectedName').textContent = '已登录：' + (data.telegram.name || data.telegram.username || 'Telegram 账号');
  $('telegram_api_id').disabled = authorized;
  $('telegram_api_hash').disabled = authorized;
  $('passwordHint').hidden = !data.telegram.password_required;
  $('groupCount').textContent = authorized ? data.group_count : '—';
  $('groupUpdated').textContent = data.groups_updated ? '更新于 ' + date(data.groups_updated) : '登录后加载';
  const cooldown = data.cooldown_until > Date.now() / 1000;
  $('telegramState').textContent = !authorized ? '待登录' : cooldown ? '冷却中' : '已连接';
  $('cooldownState').textContent = cooldown ? '等待至 ' + date(data.cooldown_until) : authorized ? '按设置间隔执行' : '等待账号连接';
  for (const kind of ['send', 'search']) {
    const sending = kind === 'send', enabled = data[kind + '_enabled'];
    $(sending ? 'sendState' : 'searchState').textContent = !enabled ? '已停止' : !authorized ? '等待登录' : cooldown ? '等待冷却' : data.busy[kind] ? '执行中' : '运行中';
    $(sending ? 'nextSend' : 'nextSearch').textContent = enabled ? '下次计划：' + date(Math.max(data['next_' + kind], data.cooldown_until)) : sending ? '每次发送一个群，循环进行' : '按关键词轮流搜索';
    $(sending ? 'startSend' : 'startSearch').disabled = enabled || !authorized;
    $(sending ? 'stopSend' : 'stopSearch').disabled = !enabled;
  }
  renderGroups();
  const labels = {selected: '已选中', skipped: '未选中', joined: '已加入', pending: '等待批准', failed: '失败', limited: '限流暂停'};
  table('candidatesTable', data.candidates.map(item => {
    const chip = document.createElement('span');
    chip.className = 'status-chip ' + (Object.hasOwn(labels, item.status) ? item.status : '');
    chip.textContent = labels[item.status] || item.status;
    return [item.title, item.keyword, chip, item.reason];
  }));
  const events = document.createDocumentFragment();
  for (const event of data.events) {
    const li = document.createElement('li'), time = document.createElement('time'), message = document.createElement('span');
    li.className = event.level;
    time.dateTime = event.at;
    time.textContent = new Date(event.at).toLocaleString();
    message.textContent = event.message;
    li.append(time, message); events.append(li);
  }
  if (!data.events.length) { const li = document.createElement('li'); li.textContent = '暂无运行记录'; events.append(li); }
  $('events').replaceChildren(events);
}

async function refresh() { const data = await api('/status'); if (loggedIn) render(data); }

async function enter() {
  await loadSettings();
  loggedIn = true;
  $('loginPage').hidden = true;
  $('workspace').hidden = false;
  await refresh();
}

function action(id, callback) {
  $(id).addEventListener('click', async () => {
    const button = $(id); button.disabled = true;
    try { await callback(); } catch (error) { notice(error.message, true); }
    finally { button.disabled = false; if (loggedIn) await refresh().catch(error => notice(error.message, true)); }
  });
}

$('loginForm').addEventListener('submit', async event => {
  event.preventDefault();
  const button = event.submitter; button.disabled = true;
  try { await api('/login', 'POST', {key: $('adminKey').value}); $('adminKey').value = ''; await enter(); }
  catch (error) { notice(error.message, true); }
  finally { button.disabled = false; }
});
action('saveSettings', async () => { await save(); notice('配置已保存'); });
action('adminLogout', async () => {
  await api('/logout', 'POST'); showLogin();
  for (const id of ['api_key', 'telegram_api_hash', 'phone', 'code', 'password']) $(id).value = '';
});
action('sendCode', async () => {
  await save(); await api('/telegram/code', 'POST', {phone: $('phone').value.trim()});
  notice('验证码已申请，请查看 Telegram 或短信');
});
action('telegramSignIn', async () => {
  const data = await api('/telegram/login', 'POST', {code: $('code').value.trim(), password: $('password').value});
  $('password').value = '';
  if (data.password_required) { $('passwordHint').hidden = false; $('password').focus(); notice('请填写两步验证密码，再点击登录'); }
  else { $('phone').value = ''; $('code').value = ''; notice('Telegram 登录成功'); }
});
action('telegramLogout', async () => {
  if (!confirm('退出 Telegram 并停止发送、搜索任务？')) return;
  await api('/telegram/logout', 'POST'); notice('已退出 Telegram');
});
action('refreshGroups', async () => { await api('/groups/refresh', 'POST'); notice('群聊已更新'); });
action('loadModels', async () => {
  await save(); const data = await api('/models', 'POST'), selected = $('model').value;
  $('model').replaceChildren(new Option('请选择模型', ''));
  for (const model of data.models) modelOption(model);
  modelOption(selected); $('model').value = selected;
  notice('已加载 ' + data.models.length + ' 个模型，选择后请保存');
});
for (const kind of ['send', 'search']) {
  const suffix = kind === 'send' ? 'Send' : 'Search';
  action('start' + suffix, async () => {
    await save(); await api('/tasks/' + kind, 'POST', {enabled: true});
    notice(kind === 'send' ? '定时发送已启动' : '搜索与自动入群已启动');
  });
  action('stop' + suffix, async () => {
    await api('/tasks/' + kind, 'POST', {enabled: false}); notice('已停止，正在进行的单次请求可能需要完成');
  });
}
action('previewPrompt', async () => {
  await save(); const data = await api('/prompt-preview', 'POST');
  $('promptPreview').textContent = data.prompt; $('promptPreview').hidden = false;
});
action('showLastPrompt', async () => {
  if (!latest?.last_prompt) { notice('本次启动后还没有调用模型'); return; }
  $('promptPreview').textContent = latest.last_prompt; $('promptPreview').hidden = false;
});
$('groupFilter').addEventListener('input', renderGroups);
$('selectVisible').addEventListener('click', () => { for (const group of visibleGroups()) selectedGroups.add(group.id); renderGroups(); });
$('clearSelection').addEventListener('click', () => { selectedGroups.clear(); renderGroups(); });
setInterval(async () => {
  if (!loggedIn || polling) return;
  polling = true;
  try { await refresh(); } catch (error) { notice(error.message, true); }
  finally { polling = false; }
}, 5000);
enter().catch(error => { showLogin(); if (!error.message.includes('请使用管理密钥')) notice(error.message, true); });
