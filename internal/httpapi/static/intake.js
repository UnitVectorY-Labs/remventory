// One decision per screen. Identification remains durable when the user leaves.
let flow = { kind: '', photos: [], text: '' };
let flowTimer, flowEpoch = 0;
document.body.addEventListener('identificationReady', event => {
  if (workspaceMode === 'intake' && flow.job?.id === event.detail.id && stage.querySelector('.processing-status')) openIdentification(event.detail.id);
});
for (const errorEvent of ['htmx:responseError', 'htmx:sendError', 'htmx:timeout']) document.body.addEventListener(errorEvent, event => {
  const target = event.detail.elt;
  if (target?.matches('.processing-status')) {
    target.replaceChildren(el('p', '', 'The connection was interrupted. Your photos and job are saved.'), button('Check again', () => openIdentification(flow.job.id), 'secondary'));
  }
});
const intakeNav = button('Pending items', () => showIntake(), 'nav-button', 'camera');
document.querySelector('.library-sidebar nav').insertBefore(intakeNav, document.querySelector('#review-changes'));
document.querySelector('#add-item').onclick = () => showCapture();
const previousWorkspace = setWorkspace;
setWorkspace = function(mode) {
  previousWorkspace(mode);
  document.body.classList.toggle('workflow-mode', mode === 'intake');
  intakeNav.classList.toggle('selected', mode === 'intake');
  if (mode === 'intake') {
    document.querySelector('.library-heading h1').textContent = flow.job?.item_id ? 'Find item details' : 'Add to your collection';
    document.querySelector('#collection-navigation').hidden = true;
    document.querySelector('#add-item').hidden = true;
  } else {
    clearTimeout(flowTimer); flowEpoch++;
    const url = new URL(location.href); url.searchParams.delete('identify'); history.replaceState(null, '', url);
  }
};
function flowError(host, error) { host.textContent = error.message || String(error); host.setAttribute('role', 'alert'); }
function flowScreen(step, title, description) {
  clearTimeout(flowTimer); flowEpoch++; libraryGeneration++;
  setWorkspace('intake'); stage.replaceChildren();
  const root = el('section', 'flow-screen');
  const progress = el('nav', 'flow-progress'); progress.setAttribute('aria-label', 'Add item progress');
  ['Type', 'Photos', 'Match', 'Save'].forEach((label, index) => {
    const node = el('span', index === step ? 'current' : index < step ? 'complete' : '', `${index + 1}. ${label}`);
    if (index === step) node.setAttribute('aria-current', 'step'); progress.append(node);
  });
  const heading = el('header', 'flow-heading'); heading.append(el('h2', '', title));
  if (description) heading.append(el('p', 'muted', description));
  const body = el('div', 'flow-body'), actions = el('footer', 'flow-actions');
  root.append(progress, heading, body, actions); stage.append(root);
  return { root, body, actions, epoch: flowEpoch };
}
function sourceLink(card) {
  const a = el('a', 'source-link', `${card.source || 'Source'} ↗`);
  if (/^https:\/\/(musicbrainz\.org|www\.wikidata\.org|amiiboapi\.org)\//.test(card.url || '')) {
    a.href = card.url; a.target = '_blank'; a.rel = 'noopener noreferrer';
  }
  return a;
}
function factCard(card, compact = false) {
  const root = el('section', compact ? 'identity-card compact' : 'identity-card'); root.dataset.layout=card.layout||'identity';
  root.append(el('h3', '', card.title));
  if (card.subtitle) root.append(el('p', 'identity-subtitle', card.subtitle));
  const facts = el('dl', 'identity-facts');
  for (const fact of (card.facts || []).slice(0, 4)) { facts.append(el('dt', '', fact.label), el('dd', '', fact.value)); }
  root.append(facts, sourceLink(card)); return root;
}
function jobPhoto(job) {
  const frame = el('div', 'flow-picture');
  const p = job.photos?.[0];
  if (p) {
    const image = el('img'); image.alt = job.result?.title || 'Your photographed item';
    image.src = `/api/intake/${job.id}/photos/${p.id}/original`;
    image.onclick = () => showItemImage(image.alt, image.src); frame.append(image);
    if (job.photos.length > 1) {
      const photos = el('div', 'photo-switcher'); photos.setAttribute('role', 'group'); photos.setAttribute('aria-label', 'Your uploaded photos');
      for (const [index, photo] of job.photos.entries()) {
        const pick = button(String(index + 1), () => {
          image.src = `/api/intake/${job.id}/photos/${photo.id}/original`;
          photos.querySelectorAll('button').forEach(b => b.setAttribute('aria-pressed', String(b === pick)));
        }, 'secondary');
        pick.setAttribute('aria-label', `View photo ${index + 1}`); pick.setAttribute('aria-pressed', String(index === 0)); photos.append(pick);
      }
      frame.append(photos);
    }
  } else frame.append(icon('box'));
  return frame;
}
function rememberJob(id) { const url = new URL(location.href); url.searchParams.set('identify', id); history.replaceState(null, '', url); }
function releaseDraft() { for (const p of flow.photos || []) URL.revokeObjectURL(p.url); }
function showCapture(kind = '') {
  if (activeController) return;
  releaseDraft(); flow = { kind, photos: [], text: '' };
  const url = new URL(location.href); url.searchParams.delete('identify'); history.replaceState(null, '', url);
  if (kind) return photoStep();
  return typeStep();
}
function typeStep() {
  const screen = flowScreen(0, 'What are you adding?', 'Choose the kind of thing. Remy will find the details.');
  const options = el('div', 'kind-grid');
  for (const [name, hint] of [['Amiibo', 'Nintendo figures and cards'], ['Vinyl', 'Records and pressings'], ['Movies', 'DVD, Blu-ray and box sets'], ['Video games', 'Games and editions'], ['Hardware', 'Consoles and accessories'], ['Other', 'Anything else']]) {
    const choice = button('', () => { flow.kind = name; photoStep(); }, 'kind-choice');
    choice.append(el('strong', '', name), el('span', 'muted', hint)); options.append(choice);
  }
  screen.body.append(options);
  screen.actions.append(button('Back to collection', () => showLibrary()), button('Enter details myself', () => openCatalog(), 'plain-button'));
}
function photoStep() {
  const screen = flowScreen(1, `Photograph your ${flow.kind === 'Amiibo' ? 'Amiibo' : 'item'}`, flow.kind === 'Amiibo' ? 'Show the character, pose and base. A second picture of the packaging can help distinguish versions.' : 'Start with the front. Add the back, label or barcode if it helps identify the edition.');
  const files = el('input'); files.type = 'file'; files.multiple = true; files.accept = 'image/jpeg,image/png,image/gif'; files.hidden = true; files.setAttribute('aria-label', 'Choose item photos');
  const camera = el('input'); camera.type = 'file'; camera.accept = 'image/*'; camera.setAttribute('capture', 'environment'); camera.hidden = true;
  const photos = el('div', 'capture-strip'), status = el('p', 'flow-status'); status.setAttribute('role', 'status');
  const next = button('Find this item →', uploadDraft, '');
  const render = () => {
    photos.replaceChildren(); next.disabled = !flow.photos.length;
    for (const [index, p] of flow.photos.entries()) {
      const box = el('div', 'capture-thumb'), image = el('img'); image.src = p.url; image.alt = p.file.name;
      box.append(image, button('Remove', () => { URL.revokeObjectURL(p.url); flow.photos.splice(index, 1); render(); }, 'plain-button')); photos.append(box);
    }
  };
  const add = input => {
    for (const file of Array.from(input.files || [])) {
      if (flow.photos.length >= 8) { status.textContent = 'Use up to eight photos of one item.'; break; }
      if (file.size > 12 * 1024 * 1024) { status.textContent = 'Each photo needs to be under 12 MB.'; continue; }
      flow.photos.push({ file, url: URL.createObjectURL(file) });
    }
    input.value = ''; render();
  };
  files.onchange = () => add(files); camera.onchange = () => add(camera);
  const capture = el('div', 'capture-options'); capture.append(button('Take a photo', () => camera.click(), '', 'camera'), button('Choose photos', () => files.click(), 'secondary', 'photo'), files, camera);
  const clue = el('details', 'optional-clue'); clue.append(el('summary', '', 'Add a clue (optional)'));
  const text = el('textarea'); text.rows = 2; text.placeholder = 'A name, series, edition or anything Remy should check'; text.value = flow.text; text.oninput = () => flow.text = text.value; clue.append(text);
  screen.body.append(capture, photos, clue, status);
  screen.actions.append(button('← Change type', () => typeStep()), next); render();
  async function uploadDraft() {
    next.disabled = true; status.textContent = 'Saving your photos…';
    const body = new FormData(); body.append('category', flow.kind); body.append('text', flow.text); flow.photos.forEach(p => body.append('images', p.file));
    try { const job = await api('/api/intake', { method: 'POST', body }); releaseDraft(); flow.photos = []; await openIdentification(job.id); }
    catch (e) { flowError(status, e); next.disabled = false; }
  }
}
async function showIntake() {
  const screen = flowScreen(-1, 'Pending items', 'Pick up where you left off, or photograph another item.');
  screen.root.querySelector('.flow-progress').hidden = true;
  document.querySelector('.library-heading h1').textContent = 'Pending items';
  screen.actions.append(button('Back to collection', () => showLibrary()), button('Add an item', () => showCapture(), ''));
  const list = el('div', 'pending-list'); screen.body.append(list); let page = 0, previous = '';
  async function refresh() {
    try {
      const data = await api('/api/intake'); if (screen.epoch !== flowEpoch) return;
      const jobs = (data.jobs || []).filter(j => ['queued', 'processing', 'review', 'failed'].includes(j.state));
      const signature = JSON.stringify([page, jobs.map(j => [j.id, j.state, j.result?.title, j.photos?.[0]?.id])]);
      if (signature === previous) { flowTimer = setTimeout(refresh, 5000); return; }
      previous = signature; list.replaceChildren();
      if (!jobs.length) list.append(el('p', 'empty', 'Everything is up to date.'));
      for (const j of jobs.slice(page * 6, page * 6 + 6)) {
        const row = button('', () => openIdentification(j.id), 'pending-row');
        const p = j.photos?.[0]; if (p) { const image = el('img'); image.src = `/api/intake/${j.id}/photos/${p.id}/thumbnail`; image.alt = ''; row.append(image); }
        const info = el('span'); info.append(el('strong', '', j.result?.title || j.input?.text?.split('\n')[0] || j.category || 'Photographed item'), el('small', '', j.state === 'review' ? 'Ready to check →' : j.state === 'failed' ? 'Needs another try →' : 'Remy is identifying it…')); row.append(info); list.append(row);
      }
      if (jobs.length > 6) { const nav = el('div', 'actions'); const prev = button('Previous', () => { page--; refresh(); }); prev.disabled = !page; const next = button('Next', () => { page++; refresh(); }); next.disabled = (page + 1) * 6 >= jobs.length; nav.append(prev, el('span', 'muted', `${page + 1} / ${Math.ceil(jobs.length / 6)}`), next); list.append(nav); }
      flowTimer = setTimeout(refresh, 5000);
    } catch (e) { flowError(list, e); }
  }
  refresh();
}
async function openIdentification(id) {
  clearTimeout(flowTimer); rememberJob(id);
  const screen = flowScreen(2, 'Remy is identifying your item', 'Your photos are saved. You can leave this screen and come back later.');
  let job;
  try { job = await api(`/api/intake/${id}?compact=1`); } catch (e) { flowError(screen.body, e); return; }
  if (screen.epoch !== flowEpoch) return;
  if(flow.job?.id!==job.id){delete flow.customTitle;delete flow.personalAttributes;}
 flow.job = job; flow.kind = job.category; flow.cards = []; flow.selected = [];
 document.querySelector('.library-heading h1').textContent=job.item_id?'Find item details':'Add to your collection';
  if (job.state === 'approved') return savedStep(job);
  if (job.state === 'review') return matchStep(job);
  if (['rejected', 'search_only'].includes(job.state)) {
    screen.body.append(jobPhoto(job), el('p', '', job.state === 'search_only' ? 'Kept as an identification without adding another owned item.' : 'This identification was discarded.'));
  } else if (job.state === 'failed') {
    screen.body.append(el('p', '', 'Remy could not finish identifying this item. Your photos are saved.'), button('Try again', () => jobAction(job, 'retry')));
  } else {
    const status = el('div', 'processing-status');
    status.setAttribute('hx-get', `/api/intake/${id}/status`);
    status.setAttribute('hx-trigger', 'load'); status.setAttribute('hx-swap', 'outerHTML'); status.setAttribute('hx-request', JSON.stringify({timeout:15000}));
    status.setAttribute('role', 'status'); status.append(el('span', 'photo-analysis-spinner'), el('p', '', 'Remy is identifying your item. Your photos are saved.'));
    screen.body.append(jobPhoto(job), status);
    if (window.htmx) htmx.process(status);
  }
  screen.actions.append(button('Pending items', () => showIntake()), button('Photograph another', () => showCapture(job.category), ''));
}
async function matchStep(job) {
  const screen = flowScreen(2, 'Is this the right item?', 'Check the name, series and version. Choose another suggestion if it fits better.');
  screen.body.append(jobPhoto(job), el('div', 'processing-status', 'Preparing the relevant details…'));
  let cards;
  try { cards = (await api(`/api/intake/${job.id}/cards`)).cards || []; } catch (e) { flowError(screen.body, e); return; }
  if (screen.epoch !== flowEpoch) return;
  flow.job = job; flow.cards = cards;
  const rank={confident:0,partial:1,ambiguous:2,rejected:3};
 const available = (job.result.matches || []).map((m, index) => ({ ...m, index })).sort((a,b)=>(rank[a.confidence]??2)-(rank[b.confidence]??2));
  const parts = available.length > 1 && available.every(m => m.role === 'component');
  if (!available.length) {
    screen.body.replaceChildren(jobPhoto(job), el('p', '', 'I couldn’t confirm a catalogue match yet. Try a clearer view of the base, label or packaging, or search by name.'));
    screen.actions.append(button('Try again', () => jobAction(job, 'retry')), button('Search by name', () => searchStep(job), ''));
    if(!job.item_id) screen.body.append(button('Enter a name and save without a match', () => { flow.selected = []; saveStep(job); }, 'plain-button'));
  } else if (parts) {
    screen.root.querySelector('h2').textContent = 'Check what is inside';
    const layout = el('div', 'match-layout'), contents = el('section', 'identity-card');
    contents.append(el('h3', '', job.result.title), el('p', 'muted', 'These are the works inside your set. The retail edition may still be unknown.'));
    const selected = new Set(available.filter(m => m.confidence !== 'rejected').map(m => m.index));
    for (const m of available) { const card = cards[m.index]; const label = el('label', 'contained-choice'), check = el('input'); check.type = 'checkbox'; check.checked = selected.has(m.index); check.onchange = () => check.checked ? selected.add(m.index) : selected.delete(m.index); const name = el('span'); name.append(el('strong', '', card?.title || m.candidate.label)); const date = card?.facts?.find(f => f.label === 'Original release')?.value; if (date || card?.subtitle) name.append(el('small', 'muted', [card?.subtitle, date?.slice(0, 4)].filter(Boolean).join(' · '))); label.append(check, name); contents.append(label); }
    layout.append(jobPhoto(job), contents); screen.body.replaceChildren(layout);
    screen.actions.append(button('Search for another title', () => searchStep(job)), button('Review and save →', () => { flow.selected = [...selected]; saveStep(job); }, ''));
  } else {
    let option = 0;
    function render() {
      const m = available[option], card = cards[m.index] || { title: m.candidate.label, facts: [], source: m.candidate.provider, url: m.candidate.uri };
      const layout = el('div', 'match-layout'), detail = factCard(card);
      const label = { confident: 'Looks like a match', partial: 'Please check the version', ambiguous: 'Several versions look similar', rejected: 'Another version you can choose' }[m.confidence] || 'Please check this match';
      detail.prepend(el('p', 'match-certainty', label));
      if(m.confidence==='rejected')detail.append(el('p','match-explanation','Remy suggested another version, but you can choose this one if it matches your item.'));
 else if (m.reason) detail.append(el('p', 'match-explanation', m.reason.length > 180 ? m.reason.slice(0, 177) + '…' : m.reason));
      if (available.length > 1) { const choices = el('div', 'variant-controls'), prev = button('←', () => { option--; render(); }), next = button('→', () => { option++; render(); }); prev.setAttribute('aria-label', 'Previous version'); next.setAttribute('aria-label', 'Next version'); prev.disabled = !option; next.disabled = option === available.length - 1; choices.append(prev, el('span', '', `Version ${option + 1} of ${available.length}`), next); detail.append(choices); }
      layout.append(jobPhoto(job), detail); screen.body.replaceChildren(layout);
      if (available.length > 1) {
        const options = el('div', 'match-options'); options.setAttribute('role', 'group'); options.setAttribute('aria-label', 'Suggested matches');
        for (const [position, alternative] of available.entries()) {
          const c = cards[alternative.index] || {title:alternative.candidate.label};
          const choice = button('', () => { option = position; render(); }, 'match-option' + (position === option ? ' selected' : ''));
          choice.setAttribute('aria-pressed', String(position === option));
          choice.append(el('strong', '', c.title), el('span', '', c.subtitle || alternative.candidate.description || 'Check this version'));
          options.append(choice);
        }
        screen.body.append(options);
      } screen.actions.replaceChildren(button('Find a different match', () => searchStep(job)), button('Use this match →', () => { flow.selected = [m.index]; saveStep(job); }, ''));
      ownershipNote(job, screen.body, screen.epoch);
    }
    render();
  }
  screen.body.append(button('Discard this identification', () => jobAction(job, 'reject'), 'plain-button quiet-action'));
}
async function ownershipNote(job, host, epoch) {
  const note = el('div', 'ownership-note'); host.append(note);
  try {
    const data = await api(`/api/intake/${job.id}/ownership`); if (epoch !== flowEpoch || !note.isConnected) return;
    const seen = new Set(); const owned = (data.ownership || []).filter(o => { if (seen.has(o.item.id)) return false; seen.add(o.item.id); return true; });
    if (owned.length) { note.append(el('strong', '', 'You may already own this'), el('p', '', owned.slice(0, 2).map(o => o.item.title).join(' · '))); if (!job.item_id) note.append(button('Keep this as a search only', () => jobAction(job, 'search_only'), 'plain-button')); }
  } catch (_) { /* Ownership hints must not prevent reviewing the identity. */ }
}
function saveStep(job) {
  const screen = flowScreen(3, job.item_id ? 'Save these details?' : 'Add this to your collection?', job.item_id ? 'Your personal notes and photos stay with the item. The selected catalogue records will be linked to it.' : 'Your photo and the selected catalogue match will be kept together.');
  const title = el('input'); const chosen=flow.cards[flow.selected[0]];
 const suggested=/amiibo/i.test(job.category)&&chosen?`${chosen.title}${chosen.subtitle?' — '+chosen.subtitle:''} amiibo`:job.result.title||chosen?.title||'';
 title.value = flow.customTitle ?? suggested;title.oninput=()=>flow.customTitle=title.value; title.setAttribute('aria-label', 'Item name');
  const layout = el('div', 'match-layout'), detail = el('section', 'identity-card');
  if (job.item_id) detail.append(el('h3', '', job.input.text.split('\n')[0] || job.result.title)); else detail.append(labeled('Name in your collection', title));
  if (!flow.selected.length) detail.append(el('p', 'muted', 'No external catalogue record will be linked yet.'));
  for (const index of flow.selected) { const c = flow.cards[index]; if (!c) continue; detail.append(factCard(c, true)); }
  layout.append(jobPhoto(job), detail); screen.body.append(layout);
  let personalEditor, replaceMatch;
  if (job.item_id) {
    replaceMatch = el('input'); replaceMatch.type = 'checkbox';
    const replace = labeled('Replace previously linked catalogue matches with this selection', replaceMatch);
    screen.body.append(replace);
  }
  if (!job.item_id) {
    const personal = el('details', 'personal-notes'); personal.append(el('summary', '', 'Add your own details (optional)'));
    personalEditor = fieldSetEditor(flow.personalAttributes || {}, [], ['title']);
    personal.append(el('p', 'muted', 'Location, condition and notes describe your own item. Catalogue facts stay linked to their source.'), personalEditor.root); screen.body.append(personal);
  }
  const status = el('p', 'flow-status'); status.setAttribute('role', 'status'); screen.body.append(status);
  const save = button(job.item_id ? 'Save match' : 'Add to my inventory', async () => {
    save.disabled = true; status.textContent = 'Saving…';
    try {
      if (personalEditor) flow.personalAttributes = personalEditor.read();
      // This click explicitly confirms the concrete selection shown on this step.
      const pending = (await api('/api/proposals?limit=100')).proposals || [];
      for (const p of pending.filter(p => p.type === 'identification_link' && p.proposed_payload.job_id === job.id)) await api(`/api/proposals/${p.id}/decision`, { method: 'POST', body: JSON.stringify({ approve: false, reason: 'Superseded by the reviewed selection.' }) });
      const payload = { replace_existing: !!replaceMatch?.checked, confirm_alternative:flow.selected.some(index=>job.result.matches[index].confidence==='rejected'), selected: flow.selected, title: title.value, tags: [job.category || 'Other'], attributes: { ...(flow.personalAttributes || {}), ...(title.value ? { title: title.value } : {}) } };
      const proposal = await api(`/api/intake/${job.id}/proposal`, { method: 'POST', body: JSON.stringify(payload) });
      await api(`/api/proposals/${proposal.id}/decision`, { method: 'POST', body: JSON.stringify({ approve: true }) });
      await loadLibraryMeta(); const saved = await api(`/api/intake/${job.id}?compact=1`); flow.savedTitle=job.item_id?job.input.text.split('\n')[0]:title.value; savedStep(saved);
    } catch (e) { try {const current=await api(`/api/intake/${job.id}?compact=1`);if(current.state==='approved'){savedStep(current);return}}catch(_){} flowError(status, e); save.disabled = false; }
  }, '');
  screen.actions.append(button('← Back to match', () => { if (personalEditor) flow.personalAttributes = personalEditor.read(); matchStep(job); }), save);
}
function savedStep(job) {
  const screen = flowScreen(3, 'Saved to your collection', 'Your item is saved. View it, or photograph the next one.');
  const name=el('h3','',flow.savedTitle||'Your item');screen.body.append(jobPhoto(job),name);
 api(`/api/items/${job.item_id}`).then(data=>{if(name.isConnected)name.textContent=(data.item||data).title||'Your item'}).catch(()=>{});
  screen.actions.append(button('View item', async () => { const item = await api(`/api/items/${job.item_id}`); setWorkspace('collection'); openItemDetail(item.item || item, {}); }), button(`Add another ${job.category === 'Amiibo' ? 'Amiibo' : 'item'}`, () => showCapture(job.category), ''));
}
async function jobAction(job, action) { try { await api(`/api/intake/${job.id}/action`, { method: 'POST', body: JSON.stringify({ action }) }); if (action === 'retry') openIdentification(job.id); else showIntake(); } catch (e) { renderError(e); } }
function searchStep(job) {
  const screen = flowScreen(2, 'Find the right catalogue record', 'Search by the character or title. Choose the specific version that matches your photo.');
  const form = el('form', 'match-search'), provider = el('select'), query = el('input'); provider.setAttribute('aria-label', 'Catalogue'); query.setAttribute('aria-label', 'Character or title'); query.placeholder = 'For example, Mario or Link';
  const names = [['amiibo', 'Amiibo catalogue'], ['musicbrainz', 'Music releases'], ['wikidata', 'Films, games and more']];
  names.forEach(([value, label]) => { const option = el('option', '', label); option.value = value; provider.append(option); });
  provider.value = /amiibo/i.test(job.category) ? 'amiibo' : /vinyl|music|record/i.test(job.category) ? 'musicbrainz' : 'wikidata';
  const submit = el('button', '', 'Search'); submit.type = 'submit'; form.append(provider, query, submit); const results = el('div', 'search-options'); screen.body.append(form, results);
  form.onsubmit = async event => {
    event.preventDefault(); submit.disabled = true; results.replaceChildren(el('p', 'muted', 'Searching…'));
    try { const data = await api(`/api/authorities/search?provider=${provider.value}&q=${encodeURIComponent(query.value)}`); if (screen.epoch !== flowEpoch) return; results.replaceChildren(); for (const c of (data.candidates || []).slice(0, 8)) { const choice = button('', async () => { results.replaceChildren(el('p', 'muted', 'Checking that record…')); try { await api(`/api/intake/${job.id}/resolve`, { method: 'POST', body: JSON.stringify({ provider: c.provider, kind: c.kind, id: c.id }) }); openIdentification(job.id); } catch (e) { flowError(results, e); } }, 'search-option'); choice.append(el('strong', '', c.label), el('small', '', c.description)); results.append(choice); } if (!data.candidates?.length) results.append(el('p', '', 'No matches. Try just the character or title, without the year or format.')); }
    catch (e) { flowError(results, e); } finally { submit.disabled = false; }
  };
  screen.actions.append(button('← Back', () => matchStep(job)));
}
async function openEnrichment() {
  const screen = flowScreen(2, 'Find details for existing items', 'Choose an item in your collection and use “Find a match”. Each result is reviewed in the same short workflow.');
  screen.actions.append(button('Open collection', () => showLibrary(), ''));
}
async function researchItem(item) {
  flow.job={item_id:item.id};
 const screen = flowScreen(2, 'Starting identification', item.title);
  try { const data = await api('/api/intake/enrich', { method: 'POST', body: JSON.stringify({ item_id: item.id }) }); openIdentification(data.jobs[0].id); }
  catch (e) { flowError(screen.body, e); }
}
// Inventory detail prioritises curated identity. Personal editing stays available.
itemDetail = function(data) {
  const item = data.item || {}, card = el('article', 'curated-item');
  card.append(el('h2', '', item.title || 'Your item'));
  const layout = el('div', 'match-layout'), photo = el('div', 'flow-picture'), information = el('div');
  if (item.images?.length) { const p = item.images.find(p => p.is_primary) || item.images[0], image = el('img'); image.src = p.original_url || `/api/images/${p.id}/original`; image.alt = item.title; image.onclick = () => showItemImage(item.title, image.src); photo.append(image); }
  const tools = el('div', 'item-detail-tools'); tools.append(button('← Collection', () => showLibrary(), 'plain-button'), button('Edit my details', () => openCatalog(item), 'secondary'), button('Correct catalogue match', () => researchItem(item), 'plain-button')); card.append(tools);
  layout.append(photo, information); card.append(layout); information.append(el('p', 'muted', 'Loading the linked identity…'));
  api(`/api/items/${item.id}/cards`).then(data => {
    information.replaceChildren(); const cards = data.cards || [];
    if (!cards.length) information.append(el('p', '', 'Link this item to a catalogue record to fill in its details.'), button('Find a match', () => researchItem(item), ''));
    else if (cards.length === 1) information.append(factCard(cards[0]));
    else { information.append(el('h3', '', 'In this collection')); for (const c of cards) information.append(button(c.title + (c.facts.find(f => f.label === 'Original release')?.value ? ` · ${c.facts.find(f => f.label === 'Original release').value.slice(0,4)}` : ''), () => openCanonical(c.entity_id), 'contained-link')); }
    const own = el('section', 'own-copy');
    const fields = item.attributes || {}, facts = [];
    const format = fields.format || (item.tags || []).filter(t => /^(dvd|blu.?ray|uhd|uhd blu.?ray|vhs|cd|cassette|lp|4k)$/i.test(t)).join(' · ');
    if (format) facts.push(['Format', format]);
    for (const key of (cards.length ? ['condition', 'location'] : ['artist_name', 'release_year', 'year', 'condition', 'location'])) {
      if (fields[key] !== undefined && fields[key] !== '' && facts.length < 4) facts.push([graphVocabulary.fields?.find(f => f.key === key)?.label || prettyKey(key), fields[key]]);
    }
    if (facts.length) { const dl = el('dl', 'identity-facts'); for (const [label, value] of facts) dl.append(el('dt', '', label), el('dd', '', String(value))); own.append(el('h4', '', 'Your copy'), dl); information.append(own); }
    const notes = el('details', 'personal-notes'); notes.append(el('summary', '', 'Your notes and organization'));
    const dl = el('dl', 'identity-facts'); for (const [key, value] of Object.entries(item.attributes || {})) if (key !== 'title') appendAttributeDetail(dl, key, value); notes.append(dl, el('p', 'muted', (item.tags || []).join(' · '))); information.append(notes);

  }).catch(e => flowError(information, e));
  return card;
};
async function openCanonical(id) {
  try { const data = await api(`/api/canonical/${id}/card`); setWorkspace('collection'); stage.replaceChildren(factCard(data.card)); const owners = el('div', 'actions'); for (const item of data.owners || []) owners.append(button(`View ${item.title}`, () => openItemDetail(item, {}))); stage.append(owners); }
  catch (e) { renderError(e); }
}
const baseProposalCard = proposalCard;
proposalCard = function(p, title) { if (p.type !== 'identification_link') return baseProposalCard(p, title); const card = el('article', 'card'); card.append(el('h3', '', p.proposed_payload.title || 'Check an item match'), button('Review match', () => openIdentification(p.proposed_payload.job_id), '')); return card; };
const resumeJob = new URLSearchParams(location.search).get('identify');
if (resumeJob) Promise.resolve().then(() => openIdentification(resumeJob));
