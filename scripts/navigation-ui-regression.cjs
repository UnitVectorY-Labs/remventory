// Run with jsdom on NODE_PATH. Every response is synthetic; no server is used.
const { JSDOM } = require('jsdom');
const { readFileSync } = require('node:fs');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const path = require('node:path');
const base = path.resolve(__dirname, '../internal/httpapi/static');
const wait = () => new Promise(resolve => setTimeout(resolve, 20));

function workspace(url) {
  const dom = new JSDOM(readFileSync(path.join(base, 'index.html'), 'utf8'), { url, runScripts: 'outside-only' });
  const w = dom.window, requests = [];
  w.HTMLDialogElement.prototype.close = function () { this.open = false; };
  w.URL.revokeObjectURL = () => {};
  const item = { id: 'owned-1', title: 'My book', attributes: {}, tags: [] };
  const jobs = Array.from({ length: 250 }, (_, index) => ({ id: `job-${index}`, title: `Pending ${index}`, category: 'Books', state: 'review', photo_id: '' }));
  w.fetch = async (url, options = {}) => {
    assert(!options.method || options.method === 'GET', 'Navigation must not write data');
    requests.push(url);
    const parsed = new URL(url, w.location.origin);
    let data;
    if (url === '/api/config') data = { config: { model_configured: true, image_storage_configured: true, vision_model_configured: true } };
    else if (url === '/api/vocabulary') data = { fields: [], tag_fields: {}, templates: [] };
    else if (url === '/api/library') data = { tags: [] };
    else if (url.startsWith('/api/proposals?')) data = { proposals: [] };
    else if (parsed.pathname === '/api/items') data = { items: [item], total: 1 };
    else if (url === '/api/items/owned-1') data = item;
    else if (url === '/api/items/owned-1/cards') data = { cards: [] };
    else if (parsed.pathname === '/api/intake/pending') {
      const offset = Number(parsed.searchParams.get('offset'));
      data = { jobs: jobs.slice(offset, offset + 6), total: jobs.length, processing: false };
    } else if (/^\/api\/intake\/job-\d+\?compact=1$/.test(url)) {
      data = { id: parsed.pathname.split('/').pop(), category: 'Books', state: 'review', input: {}, photos: [], result: { matches: [] } };
    } else if (/^\/api\/intake\/job-\d+\/cards$/.test(url)) data = { cards: [] };
    else throw new Error(`Unexpected API: ${url}`);
    return { ok: true, json: async () => data };
  };
  for (const file of ['app.js', 'library.js', 'intake.js', 'catalogue.js', 'navigation.js']) {
    new vm.Script(readFileSync(path.join(base, file), 'utf8')).runInContext(dom.getInternalVMContext());
  }
  return { dom, w, requests, call: code => vm.runInContext(code, dom.getInternalVMContext()) };
}

(async () => {
  const { w, call, requests } = workspace('http://localhost/pending?page=40');
  try {
    await wait();
    assert.equal(w.document.querySelectorAll('.pending-row').length, 6);
    assert(w.document.querySelector('.pending-list').textContent.includes('40 / 42'));
    assert(requests.includes('/api/intake/pending?limit=6&offset=234'), 'Offsets above 200 must work');
    const initialHistory = w.history.length;
    await call('showIntake(39)');
    assert.equal(w.history.length, initialHistory, 'Opening the same view must not add history');
    w.document.querySelector('.pending-row').click();
    await wait();
    assert.equal(w.location.pathname, '/identify/job-234');
    w.history.back(); await wait();
    assert.equal(w.location.pathname + w.location.search, '/pending?page=40');
    w.history.forward(); await wait();
    assert.equal(w.location.pathname, '/identify/job-234');
    await call('showIntake()');
    assert.equal(w.location.pathname + w.location.search, '/pending', 'Leaving an identification must clear its URL');
    await call('showAdmin()'); assert.equal(w.location.pathname, '/admin');
    await call('showReview()'); assert.equal(w.location.pathname, '/review');
    await call('showLibrary()');
    w.document.querySelector('.inventory-card').click();
    assert.equal(w.location.pathname, '/items/owned-1');
    await call('showCapture()'); assert.equal(w.location.pathname, '/capture');
    assert(!requests.includes('/api/intake'), 'Pending navigation must never request full job history');
  } finally { w.close(); }

  for (const [url, selector] of [
    ['/pending?page=2', '.pending-row'], ['/review', '.answer'], ['/admin', '.admin-list'],
    ['/capture', '.kind-grid'], ['/items/owned-1', '.curated-item'], ['/identify/job-1', '.flow-screen'],
    ['/?identify=job-1', '.flow-screen'], ['/?q=book&tag=Books&page=2', '.inventory-grid'],
  ]) {
    const app = workspace('http://localhost' + url);
    try {
      await wait();
      // Review renders its summary using the shared response card.
      if (url === '/review') assert(app.w.document.querySelector('#stage').textContent.includes('No pending changes.'));
      else assert(app.w.document.querySelector(selector), `Direct link did not restore ${url}`);
      if (url.includes('identify=')) assert.equal(app.w.location.pathname + app.w.location.search, '/identify/job-1');
      if (url.startsWith('/?q=')) assert.equal(app.w.location.search, '?q=book&tag=Books&page=2');
    } finally { app.w.close(); }
  }
  console.log('PASS: pending summaries and pagination, workspace URLs, direct links, legacy links, filters and browser history');
})().catch(error => { console.error(error); process.exitCode = 1; });
