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
const systemTime = time => {
  const date = new NativeDate(Date.parse(time));
  return '当前 ' + [date.getHours(), date.getMinutes(), date.getSeconds()].map(n => String(n).padStart(2, '0')).join(':');
};
const modelsHidden = () => w.getComputedStyle(w.document.getElementById('modelTabs')).display === 'none';
const restingTheme = () => w.document.body.classList.contains('is-resting');
const noticeHidden = () => w.getComputedStyle(w.document.querySelector('.notice')).display === 'none';
const rightToolsHidden = () => w.getComputedStyle(w.document.querySelector('.top-right')).display === 'none';
const pendingArtVisible = () => !w.document.getElementById('pendingArt').hidden;

(async () => {
  await drain();
  assert.equal(w.document.getElementById('quietWindowsNote'), null);
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
  assert.equal(modelsHidden(), false);
  assert.equal(restingTheme(), false);
  assert.equal(noticeHidden(), false);
  assert.equal(rightToolsHidden(), false);
  assert.equal(pendingArtVisible(), false);
  // The registered clock enters quiet time without a changed overview response.
  tick('2026-10-18T13:00:00Z');
  assert.equal(restCount(), 1);
  assert.equal(selected().status, 'resting');
  assert.equal(selected().time, systemTime('2026-10-18T13:00:00Z'));
  assert.equal(modelsHidden(), true);
  assert.equal(restingTheme(), true);
  assert.equal(noticeHidden(), true);
  assert.equal(rightToolsHidden(), true);
  assert.equal(pendingArtVisible(), false);
  assert.equal(w.getComputedStyle(w.document.body).backgroundColor, 'rgb(21, 48, 59)');
  assert.equal(w.getComputedStyle(w.document.getElementById('stage')).backgroundColor, 'rgb(21, 48, 59)');
  assert.equal(w.document.querySelector('.tl-item.active .tl-status').textContent, '休息中');
  assert.equal(w.document.getElementById('srcBtn').disabled, true);
  assert.equal(app.state().pending.key, 'sleeping-art');
  assert.match(app.state().pending.html, /<svg[\s\S]*viewBox="0 0 1200 876"/);
  assert.equal(w.getComputedStyle(w.document.getElementById('stage')).bottom, '0px');
  assert.equal(w.getComputedStyle(w.document.querySelector('.fade-blur')).display, 'none');
  assert.equal(w.getComputedStyle(w.document.querySelector('.timeline-box')).backgroundColor, 'rgba(0, 0, 0, 0)');
  const scene = new JSDOM(app.state().pending.html, { runScripts: 'outside-only' });
  assert.equal(scene.window.document.documentElement.hasAttribute('data-rest-scene'), true);
  const sceneSvg = scene.window.document.querySelector('body > svg');
  assert.equal(sceneSvg.querySelectorAll('[data-rest-edge]').length, 8);
  assert.equal(sceneSvg.querySelector('#restSceneBackground .breathe'), null);
  assert.ok(sceneSvg.querySelector('#restSceneBackground .glow')); // Retain the real moonlight animation.
  // Central artwork and all its gradient/animation definitions must remain exactly as supplied.
  const artwork = sceneSvg.cloneNode(true);
  artwork.querySelectorAll('[data-rest-edge]').forEach(el => el.remove());
  artwork.querySelector('#restSceneBackground').remove();
  assert.equal(artwork.outerHTML, w.document.getElementById('sleepingArt').content.querySelector('svg').outerHTML);
  scene.window.eval(scene.window.document.querySelector('script').textContent);
  for (const [width, height] of [[1920, 1080], [1440, 800], [390, 844], [768, 600]]) {
    scene.window.innerWidth = width;
    scene.window.innerHeight = height;
    scene.window.dispatchEvent(new scene.window.Event('resize'));
    const [left, top, boxWidth, boxHeight] = sceneSvg.getAttribute('viewBox').split(' ').map(Number);
    const scale = width / boxWidth;
    assert.ok(Math.abs(boxWidth / boxHeight - width / height) < 1e-9);
    assert.ok(-left * scale >= -1e-6);
    assert.ok((1200 - left) * scale <= width + 1e-6);
    assert.ok(-top * scale >= -1e-6);
    assert.ok((876 - top) * scale <= height - (width <= 768 ? 104 : 112) + 1e-6);
    const bottom = sceneSvg.querySelector('[data-rest-edge="bottom"]');
    assert.equal(bottom.getAttribute('viewBox'), '0 875 1200 1');
    assert.ok(Math.abs(876 + Number(bottom.getAttribute('height')) - (top + boxHeight)) < 1e-9);
    for (const edge of sceneSvg.querySelectorAll('[data-rest-edge]')) {
      assert.ok(Number(edge.getAttribute('width')) >= -1e-9);
      assert.ok(Number(edge.getAttribute('height')) >= -1e-9);
      assert.equal(edge.querySelector('use').getAttribute('href'), '#restSceneBackground');
    }
  }
  scene.window.close();
  app.paint();
  app.settle();
  assert.equal(app.state().front, 'sleeping-art');
  assert.equal(w.document.getElementById('testing').hidden, true);
  app.overview(data, true);
  assert.equal(selected().status, 'resting');
  assert.equal(app.state().pending, null); // Do not reload the SVG on every API change.
  assert.equal(modelsHidden(), true);
  const separator = w.document.querySelector('.rest-clock-separator');
  assert.equal(w.document.querySelectorAll('.rest-clock-separator').length, 2);
  assert.match(w.getComputedStyle(separator).animation, /restClockBlink 1s/);
  tick('2026-10-18T13:00:01Z');
  assert.equal(selected().time, systemTime('2026-10-18T13:00:01Z'));
  assert.equal(w.document.querySelector('.tl-item.active .tl-time').textContent, selected().time);
  assert.equal(w.document.querySelector('.rest-clock-separator'), separator);
  assert.equal(app.state().pending, null); // Ticking must not reload the SVG.
  tick('2026-10-18T13:00:59Z');
  assert.equal(selected().time, systemTime('2026-10-18T13:00:59Z'));
  tick('2026-10-18T13:01:00Z');
  assert.equal(selected().time, systemTime('2026-10-18T13:01:00Z'));
  assert.equal(w.document.querySelector('.tl-item.active .tl-time').textContent, selected().time);
  app.select(0, false);
  await drain();
  assert.equal(selected().id, 11);
  assert.equal(modelsHidden(), false);
  assert.equal(restingTheme(), false);
  assert.equal(noticeHidden(), false);
  assert.equal(rightToolsHidden(), false);
  assert.equal(w.getComputedStyle(w.document.body).backgroundColor, 'rgb(247, 245, 237)');
  assert.equal(w.getComputedStyle(w.document.getElementById('stage')).bottom, '112px');
  tick('2026-10-18T13:01:01Z');
  assert.equal(selected().id, 11); // Live seconds must not recenter historical selection.
  assert.equal(w.document.querySelector('.tl-item.s-rest .tl-time').textContent, systemTime('2026-10-18T13:01:01Z'));
  app.overview(data, true);
  assert.equal(modelsHidden(), false);
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
  assert.equal(modelsHidden(), false);
  tick('2026-10-20T13:00:00Z');
  assert.equal(modelsHidden(), true);
  assert.equal(restingTheme(), true);
  assert.equal(noticeHidden(), true);
  app.select(0, false); // Reveal model selection by viewing a real detection.
  assert.equal(modelsHidden(), false);
  w.document.querySelectorAll('#modelTabs button')[1].click();
  assert.equal(restCount(), 1);
  assert.equal(app.state().view[0].id, 13);
  assert.equal(modelsHidden(), true);
  app.group(1);
  assert.equal(app.state().view.length, 1); // Empty groups can still rest.
  assert.equal(selected().status, 'resting');
  assert.equal(w.localStorage.getItem('iq-detection:last-group-id'), '2');
  app.overview(data, true);
  assert.equal(w.document.getElementById('groupLabel').textContent, 'Second');
  tick('2026-10-21T00:00:00Z');
  assert.equal(app.state().view.length, 0);
  assert.equal(modelsHidden(), false);
  assert.equal(restingTheme(), false);
  assert.equal(noticeHidden(), false);
  assert.equal(w.document.querySelector('#tlTrack').textContent, '暂无检测记录');
  const noWindows = JSON.parse(JSON.stringify(data));
  noWindows.settings.quiet_windows = [];
  tick('2026-10-21T13:00:00Z');
  app.overview(noWindows, true);
  assert.equal(restCount(), 0); // Live settings changes remove the point.
  app.overview({ settings: data.settings, groups: [] }, true);
  assert.equal(restCount(), 0); // No visible groups must not get fake results.
  assert.equal(pendingArtVisible(), false);
  const reviewData = {
    settings: { models: ['a'], quiet_windows: [] },
    groups: [{ id: 1, name: 'Reviews', results: [
      { ...result(21, 'a'), pelican_review: 'pending', candy_review: 'approved' },
      { ...result(22, 'a'), pelican_review: 'approved', candy_review: 'pending' },
      { ...result(23, 'a'), pelican_review: 'rejected' },
      { ...result(24, 'a'), status: 'failed' },
      { ...result(25, 'a'), status: 'running' }
    ] }, { id: 2, name: 'Empty', results: [] }]
  };
  const angryFrame = w.document.getElementById('pendingArt');
  const srcdocProperty = Object.getOwnPropertyDescriptor(w.HTMLIFrameElement.prototype, 'srcdoc');
  let animationLoads = 0;
  Object.defineProperty(angryFrame, 'srcdoc', {
    get() { return srcdocProperty.get.call(this); },
    set(value) { animationLoads++; srcdocProperty.set.call(this, value); }
  });
  app.overview(reviewData, false);
  app.group(0); // Earlier group-memory checks intentionally saved the empty group.
  assert.equal(pendingArtVisible(), false); // Running is not pending review.
  app.select(0, false);
  assert.equal(selected().status, 'pending');
  assert.equal(pendingArtVisible(), true);
  assert.equal(rightToolsHidden(), false);
  assert.equal(w.document.getElementById('candy').getAttribute('aria-label'), '检测结果：通过');
  assert.equal(angryFrame.getAttribute('sandbox'), '');
  assert.equal(w.getComputedStyle(angryFrame).pointerEvents, 'none');
  assert.ok(Number(w.getComputedStyle(angryFrame).zIndex) < Number(w.getComputedStyle(w.document.querySelector('.bar')).zIndex));
  const angryDoc = new JSDOM(angryFrame.srcdoc);
  assert.equal(angryDoc.window.document.querySelector('svg').outerHTML, w.document.getElementById('angryArt').content.querySelector('svg').outerHTML);
  assert.equal(angryDoc.window.document.querySelectorAll('script').length, 0);
  assert.equal(angryDoc.window.getComputedStyle(angryDoc.window.document.body).backgroundColor, 'rgba(0, 0, 0, 0)');
  angryDoc.window.close();
  const loadsBeforePoll = animationLoads;
  app.overview(reviewData, true);
  assert.equal(pendingArtVisible(), true);
  assert.equal(animationLoads, loadsBeforePoll); // Polling must not restart the walk-in sequence.
  for (const index of [1, 2, 3, 4]) {
    app.select(index, false);
    assert.equal(pendingArtVisible(), false); // Approved, rejected, failed, and running.
    assert.equal(rightToolsHidden(), false);
  }
  app.select(0, false);
  assert.equal(pendingArtVisible(), true);
  const reviewed = JSON.parse(JSON.stringify(reviewData));
  reviewed.groups[0].results[0].pelican_review = 'approved';
  app.overview(reviewed, true);
  assert.equal(selected().status, 'pass');
  assert.equal(pendingArtVisible(), false); // Review changes take effect on the next overview.
  app.overview(reviewData, true);
  assert.equal(pendingArtVisible(), true);
  app.group(1);
  assert.equal(pendingArtVisible(), false);
  app.group(0);
  const restReviews = JSON.parse(JSON.stringify(reviewData));
  restReviews.settings.quiet_windows = [{ start: '00:00', end: '00:00' }];
  app.overview(restReviews, true);
  assert.equal(selected().status, 'resting');
  assert.equal(pendingArtVisible(), false);
  assert.equal(rightToolsHidden(), true);
  assert.ok(requested.every(url => !url.includes('quiet-now')));
  console.log('PASS: syntax, Beijing boundaries, live clock, rest controls/theme, original SVG/background, responsive coverage, pending animation visibility/layering/isolation/poll continuity, review updates, history, group memory');
  dom.window.close();
})().catch(error => { console.error(error); dom.window.close(); process.exitCode = 1; });
