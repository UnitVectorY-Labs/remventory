let libraryTypes = [];
let graphVocabulary = { fields: [], tag_fields: {}, templates: [] };
let libraryFilters = { q: "", category_id: "", tag: "", location: "" };
let libraryPage = 0;
let libraryGeneration = 0;
const catalogModal = document.querySelector("#catalog-modal");
const el = (tag, className, text) => { const node = document.createElement(tag); if (className) node.className = className; if (text !== undefined) node.textContent = text; return node; };
const icon = name => { const node = el("img", "icon"); node.src = `/static/icons/${name}.svg`; node.alt = ""; return node; };
const button = (text, action, className = "secondary", iconName = "") => { const node = el("button", className, text); node.type = "button"; node.onclick = action; if (iconName) node.prepend(icon(iconName)); return node; };
let workspaceMode = "collection";
function setWorkspace(mode) {
  workspaceMode = mode;
  document.querySelector(".library-heading h1").textContent = mode === "admin" ? "Administration" : mode === "review" ? "Review changes" : "My collection";
  document.querySelector("#add-item").hidden = mode === "admin";
  document.querySelector("#collection-navigation").hidden = mode === "admin";
  for (const [id, value] of [["library-home", "collection"], ["admin-home", "admin"], ["review-changes", "review"]]) {
    const nav = document.querySelector(`#${id}`); nav.classList.toggle("selected", value === mode);
    if (value === mode) nav.setAttribute("aria-current", "page"); else nav.removeAttribute("aria-current");
  }
}
document.querySelector("#admin-home").onclick = () => showAdmin();
document.querySelector("#library-home").onclick = () => { libraryFilters = { q: "", category_id: "", tag: "", location: "" }; showLibrary(); };
document.querySelector("#add-item").onclick = () => openCatalog();
document.querySelector("#review-changes").onclick = async () => {
  if (activeController) return;
  setWorkspace("review"); libraryGeneration++;
  try { await refreshPendingProposals(false); renderResponse({ summary: pendingProposals.length ? "Pending changes." : "No pending changes.", components: [] }); }
  catch (error) { renderError(error); }
};
document.querySelector("#toggle-remy").onclick = () => input.focus();

async function loadLibraryMeta() {
  const [vocabulary, facets, proposals] = await Promise.all([api("/api/vocabulary"), api("/api/library"), api("/api/proposals?limit=100")]);
  graphVocabulary = vocabulary;
  pendingProposals = proposals.proposals || [];
  libraryTypes = graphVocabulary.templates || [];
  const tags = document.querySelector("#tag-filters"); tags.replaceChildren();
  for (const tag of facets.tags || []) tags.append(button(`${tag.name} · ${tag.count}`, () => { const selected = libraryFilters.tag.split(",").filter(Boolean); libraryFilters.tag = (selected.includes(tag.name) ? selected.filter(value => value !== tag.name) : [...selected, tag.name]).join(","); showLibrary(); }, `tag ${libraryFilters.tag.split(",").includes(tag.name) ? "selected" : ""}`));
  document.querySelector("#review-count").textContent = pendingProposals.length || "";
  return facets;
}

async function showLibrary(page = 0) {
  if (activeController) return;
  setWorkspace("collection");
  const generation = ++libraryGeneration; ++requestGeneration;
  currentResponse = null; libraryPage = page;
  stage.innerHTML = '<p class="muted" role="status">Loading collection…</p>';
  try {
    const facets = await loadLibraryMeta();
    const params = new URLSearchParams({ ...libraryFilters, limit: "36", offset: String(page * 36) });
    const result = await api(`/api/items?${params}`);
    if (generation !== libraryGeneration) return;
    stage.replaceChildren();
    const toolbar = el("form", "library-toolbar");
    const search = el("input"); search.type = "search"; search.placeholder = "Search tags and fields…"; search.setAttribute("aria-label", "Search collection"); search.value = libraryFilters.q;
    const submit = el("button", "secondary", "Search"); submit.type = "submit";
    const smart = button("Ask Remy to find it", () => { if (search.value.trim()) { libraryGeneration++; askRemy(`Find in my inventory: ${search.value.trim()}`); } });
    toolbar.append(search, submit, smart);
    toolbar.onsubmit = event => { event.preventDefault(); libraryFilters.q = search.value.trim(); showLibrary(); };
    stage.append(toolbar);
    const caption = el("div", "shelf-caption");
    caption.append(el("p", "", `${result.total} ${result.total === 1 ? "item" : "items"}`));
    const active = Object.entries(libraryFilters).filter(([,value]) => value);
    if (active.length) caption.append(button("Clear filters", () => { libraryFilters = { q: "", category_id: "", tag: "", location: "" }; showLibrary(); }, "plain-button"));
    if (libraryFilters.tag) caption.append(el("span", "tag selected", libraryFilters.tag));
    stage.append(caption);
    const items = result.items || [];
    if (!items.length) {
      const empty = el("div", "library-empty"); empty.innerHTML = `<span class="empty-symbol"><img class="icon" src="/static/icons/box.svg" alt=""></span><h2>${active.length ? "No matching items" : "No items yet"}</h2>`;
      empty.append(button(active.length ? "Clear filters" : "Catalog your first item", () => active.length ? document.querySelector("#library-home").click() : openCatalog(), "")); stage.append(empty);
    } else {
      const grid = el("div", "inventory-grid");
      for (const item of items) {
        const type = libraryTypes.find(type => type.id === item.category_id) || {};
        const card = button("", () => { ++libraryGeneration; openItemDetail(item, type); }, "inventory-card");
        const picture = el("div", "inventory-picture");
        if (item.images?.length) { const img = el("img"); img.src = (item.images.find(image => image.is_primary) || item.images[0]).thumbnail_url; img.alt = item.title; img.loading = "lazy"; picture.append(img); }
        else { picture.classList.add("picture-placeholder"); picture.append(icon("box")); }
        const info = el("div", "inventory-info"); info.append(el("h2", "", item.title));
        const tags = el("div", "card-tags"); for (const tag of (item.tags || []).slice(0, 3)) tags.append(el("span", "tag", tag)); info.append(tags);
        card.append(picture, info); grid.append(card);
      }
      stage.append(grid);
    }
    if (result.total > 36) { const pager = el("div", "actions"); const previous = button("← Previous", () => showLibrary(page - 1)); previous.disabled = page === 0; const next = button("Next →", () => showLibrary(page + 1)); next.disabled = (page + 1) * 36 >= result.total; pager.append(previous, el("span", "muted", `Page ${page + 1} of ${Math.ceil(result.total / 36)}`), next); stage.append(pager); }
  } catch (error) { if (generation === libraryGeneration) renderError(error); }
}

function labeled(label, control) { const wrapper = el("label", "form-field"); wrapper.append(el("span", "", label), control); return wrapper; }
function field(value = "", type = "text") { const node = el("input"); node.type = type; node.value = value; return node; }
function dialogHeader(title) {
  const header = el("div", "card-heading"); const heading = el("h2", "", title); heading.id = "catalog-title";
  header.append(heading, button("", () => catalogModal.close(), "icon-button secondary", "x")); header.lastChild.setAttribute("aria-label", "Close"); return header;
}


function registeredFieldEditor(definition, value, onRemove) {
  const row = el("section", "registered-field"); row.dataset.fieldKey = definition.key;
  const heading = el("div", "field-heading"); heading.append(el("strong", "", definition.label), el("small", "muted", definition.data_type), button("", onRemove, "plain-button", "x")); heading.lastChild.setAttribute("aria-label", `Remove ${definition.label}`); row.append(heading);
  let read;
  if (definition.data_type === "links") {
    const links = el("div", "link-list");
    const addLink = (link = {}) => {
      const entry = el("div", "link-entry"); const name = field(link.name || ""); name.placeholder = "Name, e.g. Wikipedia"; name.setAttribute("aria-label", "Link name");
      const url = field(link.url || "", "url"); url.placeholder = "https://…"; url.setAttribute("aria-label", "Link URL"); entry.append(name, url, button("Remove link", () => entry.remove(), "plain-button", "x")); links.append(entry);
    };
    for (const link of Array.isArray(value) ? value : []) addLink(link);
    row.append(links, button("Add a link", () => addLink(), "secondary", "plus"));
    read = () => [...links.children].map(entry => { const [name, url] = entry.querySelectorAll("input"); return { name: name.value.trim(), url: url.value.trim() }; }).filter(link => link.name || link.url);
  } else {
    let control;
    if (["enum", "boolean"].includes(definition.data_type)) {
      control = el("select"); control.append(new Option("Not set", ""));
      for (const option of definition.data_type === "boolean" ? ["true", "false"] : definition.config?.options || []) control.append(new Option(String(option), String(option)));
      control.value = value === undefined ? "" : String(value);
    } else if (definition.key === "description") { control = el("textarea"); control.rows = 3; control.value = value ?? ""; }
    else control = field(definition.data_type === "json" ? JSON.stringify(value ?? null) : value ?? "", ["number", "date"].includes(definition.data_type) ? definition.data_type : "text");
    if (definition.data_type === "number") control.step = "any";
    control.setAttribute("aria-label", definition.label); row.append(control);
    read = () => control.value === "" ? undefined : definition.data_type === "number" ? Number(control.value) : definition.data_type === "boolean" ? control.value === "true" : definition.data_type === "json" ? JSON.parse(control.value) : control.value;
  }
  return { row, read };
}

function fieldSetEditor(initialValues = {}, suggestedKeys = []) {
  const root = el("div", "field-set-editor"); const rows = el("div"); const entries = new Map();
  const picker = el("select"); picker.setAttribute("aria-label", "Choose a defined field");
  function refreshPicker() { picker.replaceChildren(new Option("Add a defined field…", "")); for (const definition of graphVocabulary.fields) if (!entries.has(definition.key)) picker.append(new Option(definition.label, definition.key)); }
  function add(key, value) {
    if (entries.has(key)) return;
    const definition = graphVocabulary.fields.find(field => field.key === key); if (!definition) return;
    const entry = registeredFieldEditor(definition, value, () => { entry.row.remove(); entries.delete(key); refreshPicker(); }); entries.set(key, entry); rows.append(entry.row); refreshPicker();
  }
  picker.onchange = () => { if (picker.value) add(picker.value); };
  root.append(rows, picker);
  for (const key of new Set([...Object.keys(initialValues), ...suggestedKeys])) add(key, initialValues[key]);
  refreshPicker();
  return { root, add, keys: () => [...entries.keys()], read: () => { const values = {}; for (const [key, entry] of entries) { const value = entry.read(); if (value !== undefined) values[key] = value; } return values; } };
}

async function confirmStoredProposal(id) {
  let proposal = await api(`/api/proposals/${id}`);
  if (proposal.status === "pending") proposal = await api(`/api/proposals/${id}/decision`, { method: "POST", body: JSON.stringify({ approve: true }) });
  if (proposal.status !== "approved") throw new Error("This proposal was rejected. Review the pending changes before trying again.");
  return proposal;
}

async function openCatalog(existing = null, startingTemplate = null) {
  try { await loadLibraryMeta(); } catch (error) { renderError(error); return; }
  catalogModal.replaceChildren(dialogHeader(existing ? "Edit tags, fields, and pictures" : "Add to your collection"));
  const form = el("form", "catalog-form"); const layout = el("div", "catalog-layout"); const photos = el("section", "catalog-photo"); const details = el("section", "catalog-fields");
  const status = el("p", "form-status"); status.setAttribute("role", "status");
  const tags = field((existing?.tags || []).join(", ")); tags.placeholder = "Consoles, Wii, Nintendo";
  let values = { ...(existing?.attributes || {}) }, busy = false, identifying = null, reviewing = false, proposalID = null, savedItemID = existing?.id || null, recordSaved = false, frozen = null, primaryProposalID = null;
  const existingPhotos = [...(existing?.images || [])]; const additions = []; let primary = existingPhotos.find(image => image.is_primary)?.id || existingPhotos[0]?.id || null;
  let editor = fieldSetEditor(values, existing ? [] : ["title"]);
  const fieldsHost = el("div"); fieldsHost.append(editor.root);
  let selectedTemplate = startingTemplate;
  const templateStep = el("section", "catalog-template-step");
  const replaceValues = (next, suggested = []) => { editor = fieldSetEditor(next, suggested); fieldsHost.replaceChildren(editor.root); };
  const parseTags = () => [...new Set(tags.value.split(",").map(tag => tag.trim()).filter(Boolean))];
  const suggestTagFields = () => { for (const tag of parseTags()) for (const [name, keys] of Object.entries(graphVocabulary.tag_fields)) if (name.toLowerCase() === tag.toLowerCase()) for (const key of keys) editor.add(key); };
  tags.onchange = suggestTagFields;
  if (!existing) {
    const templates = el("select"); templates.setAttribute("aria-label", "Start from a template"); templates.append(new Option("Choose a template…", "")); for (const template of libraryTypes) templates.append(new Option(template.name, template.id));
    templates.append(new Option("Continue without a template", "none"));
    const applyTemplate = template => {
      const current = editor.read(); tags.value = [...new Set([...parseTags(), ...(template.default_tags || [])])].join(", ");
      replaceValues({ ...(template.default_values || {}), ...current }, (template.attributes || []).map(field => field.key)); suggestTagFields();
      status.textContent = "Template applied.";
    };
    templates.onchange = () => { selectedTemplate = libraryTypes.find(template => template.id === templates.value) || null; if (selectedTemplate) applyTemplate(selectedTemplate); layout.hidden = !templates.value; review.disabled = !templates.value; };
    templateStep.append(el("h3", "", "1. Start with a template"), labeled("What are you adding?", templates), el("p", "muted", "Remy will use your template’s fields and defaults when reading the photo."));
    if (startingTemplate) { templates.value = startingTemplate.id; applyTemplate(startingTemplate); }
    else layout.hidden = true;
  }
  details.append(labeled("Tags · separate with commas", tags), el("h3", "", "Fields"), fieldsHost);
  const gallery = el("div", "draft-gallery");
  function renderPhotos() {
    gallery.replaceChildren();
    const all = [...existingPhotos.map(image => ({ key: image.id, url: image.thumbnail_url || `/api/images/${image.id}/thumbnail?v=2`, name: "Saved photo" })), ...additions];
    if (!all.length) { const empty = el("div", "draft-preview"); empty.append(icon("photo"), el("h3", "", "Pictures")); gallery.append(empty); }
    for (const photo of all) {
      const tile = el("div", "draft-photo"); const img = el("img"); img.src = photo.url; img.alt = photo.name;
      tile.append(img, button(primary === photo.key ? "Primary picture" : "Use as primary", () => { primary = photo.key; renderPhotos(); }, primary === photo.key ? "primary-choice selected" : "primary-choice secondary"));
      if (photo.file) tile.append(button("Remove", () => { const index = additions.indexOf(photo); additions.splice(index, 1); URL.revokeObjectURL(photo.url); if (primary === photo.key) primary = existingPhotos[0]?.id || additions[0]?.key || null; renderPhotos(); }));
      gallery.append(tile);
    }
  }
  const files = field("", "file"); files.multiple = true; files.accept = "image/jpeg,image/png,image/gif";
  const camera = field("", "file"); camera.accept = "image/*"; camera.setAttribute("capture", "environment"); camera.hidden = true;
  const chooseFiles = selected => {
    for (const file of selected) {
      if (file.size > 12 * 1024 * 1024 || !["image/jpeg", "image/png", "image/gif"].includes(file.type)) { status.textContent = "Use JPEG, PNG, or GIF up to 12 MB per photo. Export HEIC as JPEG first."; continue; }
      const photo = { key: makeUUID(), file, url: URL.createObjectURL(file), name: file.name, uploadedID: null }; additions.push(photo); if (!primary) primary = photo.key;
    }
    renderPhotos();
    if (!existing && autoSuggest.checked && additions.length && !identifying) identify.click();
  };
  files.onchange = () => { chooseFiles(files.files); files.value = ""; }; camera.onchange = () => chooseFiles(camera.files);
  const autoSuggest = field("", "checkbox"); autoSuggest.checked = visionConfigured; autoSuggest.disabled = !visionConfigured;
  const analysisNote = el("p", "analysis-note");
  const progress = el("div", "photo-analysis-progress"); progress.hidden = true; progress.setAttribute("role", "status"); progress.setAttribute("aria-live", "polite");
  const progressTitle = el("strong"); const progressMessage = el("p");
  const spinner = el("span", "photo-analysis-spinner"); spinner.setAttribute("aria-hidden", "true");
  progress.append(spinner, progressTitle, progressMessage);
  const identify = button("Suggest details from picture", async () => {
    if (identifying) return;
    const target = additions.find(photo => photo.key === primary) || additions[0];
    if (!target) { status.textContent = "Choose a new picture for Remy to read."; return; }
    identifying = new AbortController(); identify.disabled = true; review.disabled = true; stop.hidden = false;
    const controller = identifying;
    const templatePicker = templateStep.querySelector("select"); if (templatePicker) templatePicker.disabled = true;
    progress.hidden = false; photos.setAttribute("aria-busy", "true"); identify.textContent = "Reading picture…";
    progressTitle.textContent = "Remy is reading your picture";
    progressMessage.textContent = selectedTemplate ? `Using ${selectedTemplate.name} to suggest details. You can keep editing while Remy works.` : "Finding details to suggest. You can keep editing while Remy works.";
    const waiting = setTimeout(() => { progressMessage.textContent = "Still reading your picture. Some models take a few minutes. You can stop and keep your draft."; }, 15000);
    status.textContent = "";
    const body = new FormData(); body.append("image", target.file);
    if (selectedTemplate) body.append("template_id", selectedTemplate.id);
    try {
      const suggestion = await api("/api/photos/identify", { method: "POST", body, signal: identifying.signal });
      if (controller.signal.aborted || !catalogModal.open) return;
      const next = { ...(suggestion.attributes || {}), ...editor.read() }; if (suggestion.title && !next.title) next.title = suggestion.title;
      replaceValues(next); tags.value = [...new Set([...parseTags(), ...(suggestion.tags || [])])].join(", "); suggestTagFields();
      analysisNote.replaceChildren(el("span", "", `Remy’s unverified suggestions: ${readableAnswer(suggestion.notes || "Please check the details before saving.").message}`));
      const extra = readableAnswer(suggestion.notes || "").details; if (extra.length) analysisNote.append(technicalDetails(extra, "Identification details")); status.textContent = "Review and edit Remy’s suggestions.";
    } catch (error) { status.textContent = error.name === "AbortError" ? "Stopped reading. Your draft is still here." : error.message; }
    finally { clearTimeout(waiting); identifying = null; identify.disabled = !visionConfigured; identify.textContent = "Suggest details from picture"; review.disabled = false; stop.hidden = true; progress.hidden = true; photos.setAttribute("aria-busy", "false"); if (templatePicker) templatePicker.disabled = false; }
  });
  const stop = button("Stop reading", () => identifying?.abort()); stop.hidden = true;
  identify.disabled = !visionConfigured;
  if (!visionConfigured) analysisNote.textContent = "Photo identification needs setup. You can still add pictures and enter details yourself.";
  photos.append(el("h3", "", existing ? "Pictures" : "2. Add pictures"), gallery, progress, stop, labeled("Choose pictures", files), camera, button("Take a picture", () => camera.click(), "secondary", "camera"), identify, el("p", "muted", "JPEG, PNG, GIF · 12 MB per picture"), ...(!existing ? [labeled("Let Remy suggest details when I add a photo", autoSuggest)] : []), analysisNote); renderPhotos();
  layout.append(photos, details); const summary = el("div", "catalog-review"); summary.hidden = true;
  const review = el("button", "", "Review changes →"); review.type = "submit";
  review.disabled = !existing && !startingTemplate;
  const back = button("Keep editing", () => { reviewing = false; summary.hidden = true; layout.hidden = false; templateStep.hidden = false; review.textContent = "Review changes →"; back.hidden = true; }); back.hidden = true;
  const actions = el("div", "actions catalog-actions"); actions.append(back, review); if (!existing) form.append(templateStep); form.append(layout, status, summary, actions); catalogModal.append(form);
  form.onsubmit = async event => {
    event.preventDefault(); if (busy || identifying || (!existing && !templateStep.querySelector("select").value)) return;
    if (!reviewing) {
      frozen = { operation: existing ? "update" : "create", ...(existing ? { item_id: existing.id } : {}), tags: parseTags(), attributes: editor.read(), ...(existingPhotos.some(photo => photo.id === primary) ? { primary_image_id: primary } : {}) };
      summary.replaceChildren(el("h3", "", frozen.attributes.title || frozen.tags.join(" · ") || "Item"));
      const list = el("dl", "details"); appendDetail(list, "Tags", frozen.tags.join(", "));
      for (const [key, value] of Object.entries(frozen.attributes)) appendAttributeDetail(list, key, value);
      appendDetail(list, "New pictures", additions.map(photo => photo.name + (photo.key === primary ? " (primary)" : "")).join(", "));
      if (existingPhotos.some(photo => photo.id === primary)) appendDetail(list, "Primary picture", "The saved picture selected above");
      summary.append(list);
      reviewing = true; layout.hidden = true; templateStep.hidden = true; summary.hidden = false; back.hidden = false; review.textContent = "Approve & save"; return;
    }
    busy = true; review.disabled = true; back.disabled = true; catalogModal.querySelector(".card-heading button").disabled = true;
    try {
      if (!recordSaved) {
        status.textContent = "Saving approved tags and fields…";
        if (!proposalID) { const proposal = await api("/api/proposals/item", { method: "POST", body: JSON.stringify(frozen) }); proposalID = proposal.id; }
        const approved = await confirmStoredProposal(proposalID); savedItemID = approved.proposed_payload.item_id; recordSaved = true; back.hidden = true;
      }
      for (const [index, photo] of additions.entries()) if (!photo.uploadedID) {
        status.textContent = `Saving picture ${index + 1} of ${additions.length}…`; const body = new FormData(); body.append("image", photo.file);
        const saved = await api(`/api/items/${savedItemID}/images`, { method: "POST", body }); photo.uploadedID = saved.id;
      }
      const desiredPrimary = additions.find(photo => photo.key === primary)?.uploadedID || primary;
      if (desiredPrimary && desiredPrimary !== frozen.primary_image_id) {
        if (!primaryProposalID) { const proposal = await api("/api/proposals/item", { method: "POST", body: JSON.stringify({ ...frozen, operation: "update", item_id: savedItemID, primary_image_id: desiredPrimary }) }); primaryProposalID = proposal.id; }
        await confirmStoredProposal(primaryProposalID);
      }
      busy = false; catalogModal.close(); await showLibrary(); setDialog({ icon: "celebrating", message: "Saved." });
    } catch (error) { status.textContent = recordSaved ? `Tags and fields are saved. Pictures or primary selection need another try: ${error.message}. Keep this window open to retry.` : error.message; review.textContent = "Retry saving"; }
    finally { busy = false; review.disabled = false; back.disabled = !!proposalID; catalogModal.querySelector(".card-heading button").disabled = false; }
  };
  catalogModal.oncancel = event => { if (busy) event.preventDefault(); };
  catalogModal.onclose = () => { identifying?.abort(); for (const photo of additions) URL.revokeObjectURL(photo.url); };
  if (!catalogModal.open) catalogModal.showModal();
}

function appendAttributeDetail(list, key, value) {
  const label = graphVocabulary.fields.find(field => field.key === key)?.label || prettyKey(key);
  if (graphVocabulary.fields.find(field => field.key === key)?.data_type === "links" && Array.isArray(value)) { for (const link of value) appendDetail(list, link.name, link.url); }
  else appendDetail(list, label, formatValue(value));
}

function reviewDefinition(path, payload, title, restore) {
  catalogModal.replaceChildren(dialogHeader(title)); const list = el("dl", "details");
  for (const [key, value] of Object.entries(payload)) {
    if (["operation", "category_id", "item_id", "attributes", "default_values", "config"].includes(key)) continue;
    appendDetail(list, prettyKey(key), Array.isArray(value) ? value.map(value => graphVocabulary.fields.find(field => field.key === value)?.label || value).join(", ") : value);
  }
  if (payload.operation === "delete") catalogModal.append(el("p", "callout danger-callout", path.endsWith("category") ? "Delete this template? Existing items will keep their fields and tags." : "Delete this item permanently? This cannot be undone."));
  if (Array.isArray(payload.attributes)) {
    appendDetail(list, "Starting fields", payload.attributes.map(definition => definition.label).join(", ") || "None");
  }
  const values = el("dl", "details");
  for (const [key, value] of Object.entries(payload.default_values || {})) appendAttributeDetail(values, key, value);
  if (payload.config?.options) appendDetail(list, "Choices", payload.config.options.join(", "));
  const technical = technicalDetails(payload);
  const status = el("p", "form-status"); status.setAttribute("role", "status"); let busy = false, proposalID = null;
  const approve = button(payload.operation === "delete" ? "Approve deletion" : "Approve & save", async () => {
    if (busy) return; busy = true; approve.disabled = true; edit.disabled = true; catalogModal.querySelector(".card-heading button").disabled = true;
    try { if (!proposalID) { const proposal = await api(path, { method: "POST", body: JSON.stringify(payload) }); proposalID = proposal.id; } await confirmStoredProposal(proposalID); busy = false; catalogModal.close(); await loadLibraryMeta(); if (workspaceMode === "admin") await showAdmin(); else await showLibrary(); setDialog({ icon: "celebrating", message: "Saved." }); }
    catch (error) { status.textContent = error.message; }
    finally { busy = false; approve.disabled = false; edit.disabled = !!proposalID; catalogModal.querySelector(".card-heading button").disabled = false; }
  }, "");
  const edit = button(payload.operation === "delete" ? "Cancel" : "Keep editing", restore); const actions = el("div", "actions"); actions.append(edit, approve); catalogModal.append(list, values, status, actions, technical);
  catalogModal.oncancel = event => { if (busy) event.preventDefault(); }; catalogModal.onclose = null;
}

async function openVocabularyEditor(section = "fields") {
  try { await loadLibraryMeta(); } catch (error) { renderError(error); return; }
  const content = el("div", "vocabulary-editor");
  const known = el("div", "vocabulary-list"); for (const field of graphVocabulary.fields) known.append(el("span", "tag", `${field.label} · ${field.data_type}`)); content.append(known);
  const form = el("form", "type-form"); form.append(el("h3", "", "Define a shared field"));
  const label = field(); label.required = true; const key = field(); key.required = true; key.pattern = "[a-z][a-z0-9_]{0,63}"; label.oninput = () => { if (!key.dataset.edited) key.value = label.value.toLowerCase().replace(/[^a-z0-9]+/g, "_").replace(/^_|_$/g, ""); }; key.oninput = () => { key.dataset.edited = "true"; };
  const kind = el("select"); for (const type of ["text", "number", "boolean", "date", "links", "enum"]) kind.append(new Option(type, type));
  const options = field(); options.placeholder = "For enum fields: separate choices with commas";
  const submit = el("button", "", "Review field definition"); submit.type = "submit";
  const advanced = el("details", "technical-details"); advanced.append(el("summary", "", "Advanced: field key"), labeled("Stable field key", key));
  const choices = labeled("Choices · separate with commas", options); choices.hidden = true;
  kind.onchange = () => { choices.hidden = kind.value !== "enum"; options.required = kind.value === "enum"; };
  form.append(labeled("Label", label), labeled("Value type", kind), choices, advanced, submit);
  form.onsubmit = event => { event.preventDefault(); reviewDefinition("/api/proposals/graph", { operation: "define_field", key: key.value, label: label.value, data_type: kind.value, config: kind.value === "enum" ? { options: options.value.split(",").map(value => value.trim()).filter(Boolean) } : {} }, "Review shared field", restore); };
  const association = el("form", "type-form"); association.append(el("h3", "", "Suggest fields for a tag")); const tag = field(); tag.required = true; tag.placeholder = "Wii, Games, Books…";
  const checks = el("div", "field-checks");
  const showChecks = () => { checks.replaceChildren(); const selected = Object.entries(graphVocabulary.tag_fields).find(([name]) => name.toLowerCase() === tag.value.trim().toLowerCase())?.[1] || []; for (const definition of graphVocabulary.fields) { const check = field("", "checkbox"); check.value = definition.key; check.checked = selected.includes(definition.key); checks.append(labeled(definition.label, check)); } };
  tag.onchange = showChecks; showChecks(); const save = el("button", "", "Review tag suggestions"); save.type = "submit"; association.append(labeled("Tag", tag), checks, save);
  association.onsubmit = event => { event.preventDefault(); reviewDefinition("/api/proposals/graph", { operation: "suggest_fields", tag: tag.value.trim(), field_keys: [...checks.querySelectorAll("input:checked")].map(input => input.value) }, "Review suggested fields", restore); };
  content.append(section === "tags" ? association : form);
  function restore() { catalogModal.replaceChildren(dialogHeader(section === "tags" ? "Manage tag defaults" : "New common field"), content); catalogModal.oncancel = null; }
  restore(); catalogModal.onclose = null; if (!catalogModal.open) catalogModal.showModal();
}

async function openTypeEditor(existing = null) {
  try { await loadLibraryMeta(); } catch (error) { renderError(error); return; }
  const content = el("form", "type-form"); const name = field(existing?.name || ""); name.required = true; const tags = field((existing?.default_tags || []).join(", ")); tags.placeholder = "Games, Wii";
  const description = field(existing?.description || "");
  const editor = fieldSetEditor(existing?.default_values || {}, (existing?.attributes || []).map(field => field.key)); content.append(labeled("Template name", name), labeled("Description", description), labeled("Default tags", tags), el("h3", "", "Starting fields and defaults"), editor.root);
  const submit = el("button", "", "Review template"); submit.type = "submit"; content.append(submit);
  content.onsubmit = event => { event.preventDefault(); const keys = editor.keys(); reviewDefinition("/api/proposals/category", { operation: existing ? "update" : "create", ...(existing ? {category_id: existing.id} : {}), name: name.value.trim(), description: description.value.trim(), default_tags: tags.value.split(",").map(value => value.trim()).filter(Boolean), default_values: editor.read(), attributes: keys.map(key => ({ ...graphVocabulary.fields.find(field => field.key === key), required: false })) }, "Review creation template", restore); };
  function restore() { catalogModal.replaceChildren(dialogHeader(existing ? "Edit template" : "Create a starting template"), content); catalogModal.oncancel = null; }
  restore(); catalogModal.onclose = null; if (!catalogModal.open) catalogModal.showModal();
}
if (!new URLSearchParams(location.search).get("message")) showLibrary();

async function proposePrimaryPicture(item, imageID) {
  try {
    const proposal = await api("/api/proposals/item", { method: "POST", body: JSON.stringify({ operation: "update", item_id: item.id, attributes: item.attributes || {}, tags: item.tags || [], primary_image_id: imageID }) });
    pendingProposals = pendingProposals.filter(candidate => candidate.id !== proposal.id); pendingProposals.push(proposal);
    renderResponse({ summary: "Review this picture as the primary image for the item.", components: [] });
  } catch (error) { renderError(error); }
}
function vocabularyCard(vocabulary) {
  const card = el("article", "card"); card.append(el("h2", "", "Shared fields & tag suggestions"));
  const fields = el("div", "vocabulary-list"); for (const definition of vocabulary.fields || []) fields.append(el("span", "tag", `${definition.label} · ${definition.data_type}`)); card.append(fields);
  for (const [tag, keys] of Object.entries(vocabulary.tag_fields || {})) card.append(el("p", "muted", `${tag}: ${keys.join(", ")}`));
  card.append(button("Open administration", () => showAdmin())); return card;
}

async function showAdmin() {
  if (activeController) return;
  const generation = ++libraryGeneration; ++requestGeneration;
  currentResponse = null; setWorkspace("admin");
  stage.replaceChildren(el("p", "muted", "Loading administration…"));
  try {
    await loadLibraryMeta();
    if (generation !== libraryGeneration) return;
    stage.replaceChildren(el("p", "muted", "Define the common fields, starting templates, and tag defaults used across your collection. Changes are reviewed before saving."));
    const fields = el("article", "card"); const fieldHeading = el("div", "card-heading");
    fieldHeading.append(el("h2", "", "Common fields"), button("New common field", () => openVocabularyEditor("fields"), "secondary", "plus"));
    fields.append(fieldHeading, el("p", "muted", "Reusable details such as Year, Condition, or Purchase price. Every item can use any approved field."));
    const definitions = el("div", "admin-list");
    for (const definition of graphVocabulary.fields) { const row = el("div", "admin-row"); row.append(el("strong", "", definition.label), el("span", "muted", prettyKey(definition.data_type))); definitions.append(row); }
    fields.append(definitions);
    const templates = el("article", "card"); const heading = el("div", "card-heading");
    heading.append(el("h2", "", "Templates"), button("New template", () => openTypeEditor(), "secondary", "plus"));
    templates.append(heading, el("p", "muted", "Starting fields, values, and tags for new items. Editing a template affects future drafts only."));
    if (!libraryTypes.length) templates.append(el("p", "muted", "No templates yet. Create one for belongings you add often."));
    for (const template of libraryTypes) {
      const row = el("div", "admin-row"); const info = el("div");
      info.append(el("strong", "", template.name), el("p", "muted", `${(template.attributes || []).length} starting fields · ${(template.default_tags || []).join(", ") || "No default tags"}`));
      const actions = el("div", "actions"); actions.append(button("Edit", () => openTypeEditor(template), "secondary", "pencil"), button("Delete", () => reviewDeletion("template", template), "secondary danger-text", "trash")); row.append(info, actions); templates.append(row);
    }
    const tags = el("article", "card"); const tagHeading = el("div", "card-heading");
    tagHeading.append(el("h2", "", "Tag defaults"), button("Manage tag defaults", () => openVocabularyEditor("tags"), "secondary", "settings"));
    tags.append(tagHeading, el("p", "muted", "Choose which fields appear when a tag is added to a draft. These are suggestions; values remain optional."));
    for (const [tag, keys] of Object.entries(graphVocabulary.tag_fields || {})) { const row = el("div", "admin-row"); row.append(el("strong", "", tag), el("span", "muted", keys.map(key => graphVocabulary.fields.find(field => field.key === key)?.label || prettyKey(key)).join(", "))); tags.append(row); }
    stage.append(fields, templates, tags);
  } catch (error) { if (generation === libraryGeneration) renderError(error); }
}

function reviewDeletion(kind, record) {
  const isTemplate = kind === "template";
  reviewDefinition(isTemplate ? "/api/proposals/category" : "/api/proposals/item", {
    operation: "delete", ...(isTemplate ? { category_id: record.id, name: record.name } : { item_id: record.id, title: record.title || "Untitled item" }),
  }, isTemplate ? "Review template deletion" : "Review item deletion", () => catalogModal.close());
  if (!catalogModal.open) catalogModal.showModal();
}
