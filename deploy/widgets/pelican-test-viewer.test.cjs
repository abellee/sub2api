// Run from the repository root: node deploy/widgets/pelican-test-viewer.test.cjs
// Uses the frontend's existing jsdom dependency; never contacts production APIs.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { createRequire } = require('node:module');
const { Script } = require('node:vm');
const { JSDOM } = createRequire(path.resolve(__dirname, '../../frontend/package.json'))('jsdom');
const html = fs.readFileSync(path.join(__dirname, 'pelican-test-viewer.html'), 'utf8');
const script = html.match(/<script>([\s\S]*?)<\/script>/)[1];
new Script(script); // Syntax check of the exact production script.
const dom = new JSDOM(html, { url: 'https://widget.example/pelican-test-viewer.html', runScripts: 'outside-only' });
const w = dom.window;
let now = Date.parse('2026-10-18T12:59:59Z'); // Beijing 20:59:59
const NativeDate = w.Date;
w.Date = class extends NativeDate {
  constructor(...args) { super(...(args.length ? args : [now])); }
  static now() { return now; }
};
const intervals = [];
w.setInterval = (fn, ms) => { intervals.push({ fn, ms }); return intervals.length; };
w.setTimeout = () => 1;
w.clearTimeout = () => {};
w.requestAnimationFrame = () => 1;
w.cancelAnimationFrame = () => {};
const quiet = [{ start: '21:00', end: '08:00' }];
const result = (id, model) => ({ id, model, created_at: '2026-10-18T10:00:00Z', status: 'completed', pelican_review: 'approved' });
const data = {
  settings: { models: ['a', 'b'], quiet_windows: quiet },
  groups: [
    { id: 1, name: 'First', results: [result(11, 'a'), result(12, 'a'), result(13, 'b')] },
    { id: 2, name: 'Second', results: [] }
  ]
};
const requested = [];
w.fetch = async url => {
  requested.push(url);
  return { ok: true, json: async () => url.endsWith('/overview') ? { code: 0, data } :
    { code: 0, data: { pelican_html: '<html><body>historical result</body></html>', candy_answer: '7' } } };
};
// Expose state only in the isolated test copy, leaving production code untouched.
const instrumented = script.replace('  // ---- 初始化 ----', `
  window.widgetTest = {
    inWindow: function (windows, time) { quietWindows = windows; return inQuietWindow(Date.parse(time)); },
    overview: applyOverview, refresh: refreshRestPoint, select: selectRun,
    state: function () { return { view: view, selected: view[runIdx], pending: pendingHtml, front: frontSrc }; },
    paint: flushPaint,
    settle: function () { frames.forEach(function (f) {
      // jsdom does not load srcdoc; emulate its load event after checking the payload.
      if (f.onload) { f.contentDocument.body.innerHTML = '<main>loaded</main>'; f.onload(); }
    }); },
    group: function (i) { setGroup(i); resetToLatest(); }
  };
  // ---- 初始化 ----`);
w.eval(instrumented);
const app = w.widgetTest;
const drain = async () => { for (let i = 0; i < 10; i++) await Promise.resolve(); };
const tick = time => { now = Date.parse(time); intervals.find(t => t.ms === 1000).fn(); };
const selected = () => app.state().selected;
const restCount = () => app.state().view.filter(r => r.status === 'resting').length;

(async () => {
  await drain();
  // Beijing time is independent of process/browser timezone; start inclusive, end exclusive.
  const cases = [
    [quiet, '2026-10-18T12:59:59Z', false],
    [quiet, '2026-10-18T13:00:00Z', true],
    [quiet, '2026-10-18T23:59:59Z', true],
    [quiet, '2026-10-19T00:00:00Z', false],
    [[{ start: '9:00', end: '12:00' }], '2026-10-18T01:00:00Z', true],
    [[{ start: '9:00', end: '12:00' }], '2026-10-18T04:00:00Z', false],
    [[{ start: '00:00', end: '00:00' }], '2026-10-18T08:00:00Z', true],
    [[{ start: '09:00', end: '10:00' }, ...quiet], '2026-10-18T01:30:00Z', true],
    [[], '2026-10-18T13:00:00Z', false],
    [[null, { start: '24:00', end: '08:00' }, { start: 'bad', end: '10:00' }], '2026-10-18T13:00:00Z', false]
  ];
  for (const [windows, time, expected] of cases) assert.equal(app.inWindow(windows, time), expected, time);
  app.overview(data, true);
  assert.equal(restCount(), 0);
  assert.equal(selected().id, 12);
  // The registered clock enters quiet time without a changed overview response.
  tick('2026-10-18T13:00:00Z');
  assert.equal(restCount(), 1);
  assert.equal(selected().status, 'resting');
  assert.equal(selected().time, '当前 21:00');
  assert.equal(w.document.querySelector('.tl-item.active .tl-status').textContent, '休息中');
  assert.equal(w.document.getElementById('srcBtn').disabled, true);
  assert.equal(app.state().pending.key, 'sleeping-art');
  assert.match(app.state().pending.html, /<svg[\s\S]*viewBox="0 0 1200 876"/);
  app.paint();
  app.settle();
  assert.equal(app.state().front, 'sleeping-art');
  assert.equal(w.document.getElementById('testing').hidden, true);
  app.overview(data, true);
  assert.equal(selected().status, 'resting');
  assert.equal(app.state().pending, null); // Do not reload the SVG on every API change.
  tick('2026-10-18T13:01:00Z');
  assert.equal(selected().time, '当前 21:01');
  assert.equal(w.document.querySelector('.tl-item.active .tl-time').textContent, '当前 21:01');
  app.select(0, false);
  await drain();
  assert.equal(selected().id, 11);
  assert.match(app.state().pending.html, /historical result/);
  app.paint();
  app.settle();
  tick('2026-10-19T00:00:00Z');
  assert.equal(restCount(), 0);
  assert.equal(selected().id, 11); // Historical selection survives the boundary.
  app.select(1, false);
  tick('2026-10-19T13:00:00Z');
  assert.equal(selected().status, 'resting');
  tick('2026-10-20T00:00:00Z');
  assert.equal(selected().id, 12); // Following latest returns to latest real run.
  tick('2026-10-20T13:00:00Z');
  w.document.querySelectorAll('#modelTabs button')[1].click();
  assert.equal(restCount(), 1);
  assert.equal(app.state().view[0].id, 13);
  app.group(1);
  assert.equal(app.state().view.length, 1); // Empty groups can still rest.
  assert.equal(selected().status, 'resting');
  assert.equal(w.localStorage.getItem('iq-detection:last-group-id'), '2');
  app.overview(data, true);
  assert.equal(w.document.getElementById('groupLabel').textContent, 'Second');
  tick('2026-10-21T00:00:00Z');
  assert.equal(app.state().view.length, 0);
  assert.equal(w.document.querySelector('#tlTrack').textContent, '暂无检测记录');
  const noWindows = JSON.parse(JSON.stringify(data));
  noWindows.settings.quiet_windows = [];
  tick('2026-10-21T13:00:00Z');
  app.overview(noWindows, true);
  assert.equal(restCount(), 0); // Live settings changes remove the point.
  app.overview({ settings: data.settings, groups: [] }, true);
  assert.equal(restCount(), 0); // No visible groups must not get fake results.
  assert.ok(requested.every(url => !url.includes('quiet-now')));
  console.log('PASS: syntax, Beijing window boundaries, clock transitions, SVG rendering, history, models, group memory, empty groups, settings changes, no synthetic detail requests');
  dom.window.close();
})().catch(error => { console.error(error); dom.window.close(); process.exitCode = 1; });
