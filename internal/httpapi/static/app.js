const form = document.querySelector("#remy-form");
const input = document.querySelector("#message");
const sendButton = document.querySelector("#send-request");
const newChat = document.querySelector("#new-chat");
const browseInventory = document.querySelector("#browse-inventory");
const stage = document.querySelector("#stage");
const dialog = document.querySelector("#remy-dialog");
const stopRequest = document.querySelector("#stop-request");
const submittedMessage = document.querySelector("#submitted-message");
const composerHint = document.querySelector("#composer-hint");
const configurationWarning = document.querySelector("#configuration-warning");
const remyAvatar = document.querySelector("#remy-avatar");
const imageModal = document.querySelector("#image-modal");
const imageModalContent = document.querySelector("#image-modal-content");
const imageModalTitle = document.querySelector("#image-modal-title");
const closeImageModal = document.querySelector("#close-image-modal");
let activeController = null;
let activeMessage = "";
let currentResponse = null;
let pendingProposals = [];
let conversationID = getConversationID();
let requestGeneration = 0;
const renderableComponentTypes = new Set(["category_proposal", "item_proposal", "item_detail", "category_definition", "item_list", "item_cards", "query_result", "category_list", "statistic", "grouped_statistic", "comparison", "clarification"]);

showConfigurationStatus();
refreshPendingProposals().catch(() => {});

closeImageModal.addEventListener("click", () => imageModal.close());
imageModal.addEventListener("click", (event) => {
  if (event.target === imageModal) imageModal.close();
});

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const message = input.value.trim();
  if (!message || activeController) return;
  input.value = "";
  await askRemy(message);
});

input.addEventListener("keydown", (event) => {
  if (event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();
    form.requestSubmit();
  }
});

newChat.addEventListener("click", () => {
  requestGeneration += 1;
  if (activeController) activeController.abort();
  activeController = null;
  currentResponse = null;
  activeMessage = "";
  conversationID = makeUUID();
  try { localStorage.setItem("remventory-conversation", conversationID); } catch (_) {}
  renderEmpty();
  setDialog({ icon: "ready", message: "Hi, I'm Remy. How can I help you manage your inventory." });
  setRemyImage("ready");
  input.value = "";
  input.focus();
});

browseInventory.addEventListener("click", () => showBrowseCategories());

stopRequest.addEventListener("click", () => {
  if (!activeController) return;
  stopRequest.disabled = true;
  setDialog({ icon: "thinking", message: "I’m stopping here and putting your request back in the composer." });
  activeController.abort();
});

const initialMessage = new URLSearchParams(window.location.search).get("message");
if (initialMessage) {
  input.value = initialMessage;
  form.requestSubmit();
}

async function askRemy(message) {
  const generation = ++requestGeneration;
  activeController = new AbortController();
  activeMessage = message;
  const context = visibleContext(currentResponse);
  setWorking(message);
  try {
    setDialog({ icon: "thinking", message: "I’m checking the collection and getting the useful details together." });
    const response = await api("/api/remy/request", {
      method: "POST",
      body: JSON.stringify({ message, session_id: conversationID, focus_ids: currentFocusIDs() }),
      signal: activeController.signal,
    });
    if (generation !== requestGeneration) return;
    if (response.session_id) {
      conversationID = response.session_id;
      try { localStorage.setItem("remventory-conversation", conversationID); } catch (_) {}
    }
    await refreshPendingProposals(false);
    renderResponse(response);
    setDialog({ icon: "ready", message: response.summary || "All set." });
  } catch (error) {
    if (generation !== requestGeneration) return;
    if (error.name === "AbortError") {
      renderStopped();
    } else if (generation === requestGeneration) {
      await refreshPendingProposals(false).catch(() => {});
      renderError(error);
    }
  } finally {
    if (generation === requestGeneration) clearWorking();
  }
}

async function api(path, options = {}) {
  const headers = options.body instanceof FormData ? {} : { "Content-Type": "application/json" };
  const response = await fetch(path, {
    headers,
    ...options,
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error || response.statusText);
  return body;
}

async function showConfigurationStatus() {
  try {
    const status = await api("/api/config");
    const missing = [];
    if (!status.config?.model_configured) missing.push("Remy needs OPENAI_MAIN_MODEL");
    if (!status.config?.image_storage_configured) missing.push("picture uploads need the S3-compatible storage settings");
    if (missing.length) {
      configurationWarning.hidden = false;
      configurationWarning.textContent = `${missing.join("; ")} before all features are available.`;
    }
  } catch (_) {
    // Submission errors remain close to the request surface.
  }
}

function renderResponse(response) {
  currentResponse = response;
  stage.innerHTML = "";
  if (response.summary) {
    const answer = textCard(response.summary);
    stage.append(answer);
  }
  let displayed = 0;
  const visible = [...(response.components || [])];
  const present = new Set(visible.map((component) => component?.data?.id).filter(Boolean));
  for (const proposal of pendingProposals) {
    if (present.has(proposal.id)) continue;
    visible.push({ type: proposal.type === "category_create" ? "category_proposal" : "item_proposal", data: proposal });
  }
  for (const component of visible) {
    if (component.type === "text" || component.type === "clarification") {
      const value=typeof component.data==="string"?component.data:(component.data?.text||component.data?.question||"");
      if (String(value).trim()===String(response.summary||"").trim()) continue;
    }
    const rendered = renderComponent(component);
    if (rendered) {
      stage.append(rendered);
      displayed += 1;
    }
  }
  if (!displayed && !response.summary) renderEmpty();
}

async function refreshPendingProposals(rerender = true) {
  const generation=requestGeneration;
  const result=await api("/api/proposals?limit=30");
  if(generation!==requestGeneration)return;
  pendingProposals=result.proposals||[];
  if(rerender) {
    if(currentResponse) renderResponse(currentResponse);
    else if(pendingProposals.length) renderResponse({state:"completed",summary:"Pending changes for your review.",components:[]});
  }
}

function visibleContext(response) {
  if (!response) return null;
  return { state: response.state, summary: response.summary, request_summary: response.request_summary, components: response.components || [] };
}

function bodyContentChanged(before, after) {
  return JSON.stringify(bodyComponents(before)) !== JSON.stringify(bodyComponents(after));
}

function bodyComponents(response) {
  return (response?.components || []).filter((component) => renderableComponentTypes.has(component.type));
}

function renderComponent(component) {
  switch (component.type) {
    case "category_proposal": return proposalCard(component.data, "Category change");
    case "item_proposal": return proposalCard(component.data, "Item change");
    case "item_detail": return itemDetail(component.data);
    case "category_definition": return categoryDefinition(component.data);
    case "item_list": return itemList(component.data);
    case "item_cards": return itemCards(component.data);
    case "query_result": return queryResult(component.data);
    case "category_list": return categoryList(component.data);
    case "text": case "clarification": return textCard(component.data);
    case "statistic": return statisticCard(component.data);
    case "grouped_statistic": return groupedStatisticCard(component.data);
    case "comparison": return comparisonCard(component.data);
    default: return null;
  }
}

function proposalCard(proposal, title) {
  const card = document.createElement("article");
  card.className = "card proposal";
  const payload = proposal.proposed_payload || {};
  const operation = payload.operation || "create";
  card.innerHTML = `<div class="card-heading"><div><p class="eyebrow">Confirmation required</p><h2>${escapeHTML(title)}</h2></div><span class="badge">${escapeHTML(operationLabel(operation))}</span></div>`;

  if (operation === "delete") {
    card.insertAdjacentHTML("beforeend", `<p class="callout danger-callout">${title === "Category change" ? "This will also remove all items in the category." : "This item will be permanently removed when approved."}</p>`);
  }
  const details = document.createElement("dl");
  details.className = "details";
  appendDetail(details, title === "Category change" ? "Category" : "Item", payload.name || payload.title);
  appendDetail(details, "Description", payload.description);
  appendDetail(details, "Quantity", payload.quantity);
  appendDetail(details, "Quantity change", payload.quantity_delta ? signed(payload.quantity_delta) : "");
  card.append(details);

  if (Array.isArray(payload.attributes)) card.append(attributeDefinitionTable(payload.attributes, "Attributes to track"));
  else if (payload.attributes && typeof payload.attributes === "object") {
    const before = payload.previous_attributes && typeof payload.previous_attributes === "object" ? payload.previous_attributes : null;
    card.append(attributeValueTable(payload.attributes, "Item details", before));
  }

  if (proposal.status === "pending") {
    const actions = document.createElement("div");
    actions.className = "actions";
    const approve = document.createElement("button");
    approve.textContent = operation === "delete" ? "Approve deletion" : "Approve changes";
    approve.addEventListener("click", () => decide(proposal.id, true));
    const reject = document.createElement("button");
    reject.className = "secondary";
    reject.textContent = "Reject proposal";
    reject.addEventListener("click", () => decide(proposal.id, false));
    actions.append(approve, reject);
    card.append(actions);
  } else {
    card.insertAdjacentHTML("beforeend", `<p class="decision ${proposal.status}">${proposal.status === "approved" ? "Approved and saved." : "Rejected. No inventory data was changed."}</p>`);
  }
  return card;
}

async function decide(id, approve) {
  if (activeController) return;
  const generation = ++requestGeneration;
  activeController = new AbortController();
  activeMessage = approve ? "Approve this proposal" : "Reject this proposal";
  const context = visibleContext(currentResponse);
  setWorking(activeMessage);
  try {
    setDialog({ icon: "thinking", message: "I’m recording your decision." });
    const proposal = await api(`/api/proposals/${id}/decision`, {
      method: "POST",
      body: JSON.stringify({ approve, reason: "" }),
      signal: activeController.signal,
    });
    let components = [{ type: proposal.type === "category_create" ? "category_proposal" : "item_proposal", data: proposal }];
    const payload = proposal.proposed_payload || {};
    if (approve && proposal.type === "item_change" && payload.operation !== "delete" && payload.item_id) {
      const [item, category] = await Promise.all([
        api(`/api/items/${payload.item_id}`, { signal: activeController.signal }),
        api(`/api/categories/${payload.category_id}`, { signal: activeController.signal }),
      ]);
      components = [{ type: "item_detail", data: { item, category } }];
    }
    const response = {
      state: "completed",
      summary: approve ? "Changes approved and saved." : "Proposal rejected. No inventory data was changed.",
      components,
    };
    if (generation !== requestGeneration) return;
    renderResponse(response);
    await refreshPendingProposals();
    setDialog({ icon: "ready", message: response.summary });
  } catch (error) {
    if (generation===requestGeneration && error.name === "AbortError") renderStopped(); else if (generation === requestGeneration) { await refreshPendingProposals(false).catch(()=>{}); renderError(error); }
  } finally {
    if (generation === requestGeneration) clearWorking();
  }
}

function textCard(data) {
  const card = document.createElement("article");
  card.className = "card remy-answer";
  const text = typeof data === "string" ? data : (data?.text || data?.question || "");
  for (const line of String(text).split("\n")) {
    const paragraph = document.createElement("p");
    const tokens = line.split(/(\*\*[^*]+\*\*|\*[^*]+\*)/g);
    for (const token of tokens) {
      if (token.startsWith("**") && token.endsWith("**")) { const strong=document.createElement("strong"); strong.textContent=token.slice(2,-2); paragraph.append(strong); }
      else if (token.startsWith("*") && token.endsWith("*")) { const em=document.createElement("em"); em.textContent=token.slice(1,-1); paragraph.append(em); }
      else paragraph.append(document.createTextNode(token));
    }
    card.append(paragraph);
  }
  return card;
}

function statisticCard(data) {
  const card = document.createElement("article");
  card.className = "card statistic-card";
  const heading = document.createElement("h2"); heading.textContent = data?.label || "Collection";
  const value = document.createElement("strong"); value.textContent = String(data?.value ?? "");
  const detail = document.createElement("p"); detail.textContent = data?.detail || "";
  card.append(heading, value, detail); return card;
}

function comparisonCard(data) {
  const card = document.createElement("article"); card.className = "card";
  const heading = document.createElement("h2"); heading.textContent = data?.title || "Comparison"; card.append(heading);
  const table = document.createElement("table"); const body = document.createElement("tbody");
  for (const row of data?.rows || []) { const tr = document.createElement("tr"); for (const cell of row) { const td = document.createElement("td"); td.textContent = String(cell ?? ""); tr.append(td); } body.append(tr); }
  table.append(body); card.append(table); return card;
}

function groupedStatisticCard(data) {
  const card=document.createElement("article"); card.className="card";
  const heading=document.createElement("h2"); heading.textContent=data?.label||"Grouped inventory"; card.append(heading);
  const table=document.createElement("table"); const body=document.createElement("tbody");
  const groups=data?.groups||{};
  for(const [key,value] of Object.entries(groups).sort(([a],[b])=>a.localeCompare(b))) {const row=document.createElement("tr");const label=document.createElement("th");label.textContent=key;const count=document.createElement("td");count.textContent=String(value);row.append(label,count);body.append(row);}
  table.append(body);card.append(table);return card;
}

function renderError(error) {
  const card = document.createElement("article"); card.className = "card error-card";
  const title = document.createElement("h2"); title.textContent = "I couldn’t finish that request";
  const detail = document.createElement("p"); detail.textContent = error?.message || "Please try again.";
  card.append(title, detail); stage.replaceChildren(card);
  for(const proposal of pendingProposals) stage.append(proposalCard(proposal,proposal.type==="category_create"?"Category change":"Item change"));
  setDialog({icon:"error",message:"I couldn’t confirm how that request finished. Refresh proposals or check the inventory before retrying."});
}

function getConversationID() {
  try {
    const existing = localStorage.getItem("remventory-conversation"); if (existing && /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(existing)) return existing;
    const id = makeUUID();
    localStorage.setItem("remventory-conversation", id); return id;
  } catch (_) { return makeUUID(); }
}

function makeUUID() {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID();
  const bytes = new Uint8Array(16);
  if (globalThis.crypto?.getRandomValues) globalThis.crypto.getRandomValues(bytes);
  else for (let i=0;i<bytes.length;i++) bytes[i] = Math.floor(Math.random()*256);
  bytes[6]=(bytes[6]&0x0f)|0x40; bytes[8]=(bytes[8]&0x3f)|0x80;
  const hex=Array.from(bytes,b=>b.toString(16).padStart(2,"0")).join("");
  return `${hex.slice(0,8)}-${hex.slice(8,12)}-${hex.slice(12,16)}-${hex.slice(16,20)}-${hex.slice(20)}`;
}

function currentFocusIDs() {
  const ids = new Set();
  const addMany = (values) => { if(Array.isArray(values)) for(const value of values) add(value); };
  const add = (value) => { if (typeof value === "string" && value.length < 100) ids.add(value); };
  const walk = (value) => {
    if (!value || typeof value !== "object" || ids.size >= 30) return;
    if (Array.isArray(value)) { for (const item of value) walk(item); return; }
    add(value.id); add(value.item_id); add(value.category_id);
    for (const [key, child] of Object.entries(value)) {if(key==="item_ids")addMany(child);if (key !== "images") walk(child);}
  };
  for (const component of currentResponse?.components || []) walk(component?.data);
  return Array.from(ids).slice(0,30);
}

function categoryDefinition(category) {
  const card = document.createElement("article");
  card.className = "card";
  card.innerHTML = `<div class="card-heading"><div><p class="eyebrow">Category definition</p><h2>${escapeHTML(category.name || "Category")}</h2></div></div>`;
  if (category.description) card.insertAdjacentHTML("beforeend", `<p class="summary">${escapeHTML(category.description)}</p>`);
  card.append(attributeDefinitionTable(category.attributes || [], "Attributes"));
  return card;
}

function queryResult(data) {
  const card = document.createElement("article");
  card.className = "card";
  const categories = (data.categories || []).length ? data.categories : (data.category?.id ? [data.category] : []);
  const heading = categories.length > 1 ? `Across ${categories.length} collections` : (categories[0]?.name || "Inventory");
  card.innerHTML = `<div class="card-heading"><div><p class="eyebrow">Inventory check</p><h2>${escapeHTML(heading)}</h2></div></div><p class="summary">${escapeHTML(data.summary || "")}</p>`;
  const matches = data.matches || [];
  if (matches.length && categories.length > 1) {
    for (const category of categories) {
      const categoryMatches = matches.filter((item) => item.category_id === category.id);
      if (categoryMatches.length) card.append(itemsTable(categoryMatches, category, category.name || "Matching items"));
    }
  } else if (matches.length) {
    card.append(itemsTable(matches, categories[0] || data.category || {}, "Matching items"));
  }
  return card;
}

function itemList(data) {
  const card = document.createElement("article");
  card.className = "card";
  card.innerHTML = `<div class="card-heading"><div><p class="eyebrow">Inventory</p><h2>${escapeHTML(data.category?.name || "Items")}</h2></div></div>`;
  const items = data.items || [];
  if (!items.length) card.insertAdjacentHTML("beforeend", `<p class="summary">No items yet.</p>`);
  else card.append(itemsTable(items, data.category || {}, "Items"));
  if (data.has_more) { const more=document.createElement("button"); more.type="button"; more.className="secondary"; more.textContent="Load more"; more.addEventListener("click",()=>showCategoryItems(data.category,data.offset||items.length,true)); card.append(more); }
  return card;
}

function itemCards(data) {
  const card=document.createElement("article"); card.className="card item-card-collection";
  const heading=document.createElement("h2"); heading.textContent=data?.title||"Items";card.append(heading);
  const categories=data?.categories||[];const items=data?.matches||[];
  for(const item of items) { const category=categories.find((candidate)=>candidate.id===item.category_id)||{};const detail=itemDetail({item,category});detail.classList.add("compact-item-card");card.append(detail); }
  return card;
}

function itemDetail(data) {
  const item = data.item || {};
  const category = data.category || {};
  const card = document.createElement("article");
  card.className = "card item-detail-card";
  card.innerHTML = `<div class="card-heading"><div><p class="eyebrow">${escapeHTML(category.name || "Inventory item")}</p><h2>${escapeHTML(item.title || "Item")}</h2></div><span class="badge">Quantity ${escapeHTML(String(item.quantity || 1))}</span></div>`;

  const layout = document.createElement("div");
  layout.className = "item-detail-layout";
  layout.append(itemPicturePanel(item));

  const information = document.createElement("section");
  information.className = "item-information";
  information.innerHTML = "<h3>Item details</h3>";
  const fields = document.createElement("dl");
  fields.className = "item-fields";
  appendDetail(fields, "Quantity", item.quantity);
  const values = item.attributes || {};
  for (const attribute of category.attributes || []) {
    appendDetail(fields, attribute.label || prettyKey(attribute.key), formatValue(values[attribute.key]));
  }
  information.append(fields);
  layout.append(information);
  card.append(layout);
  return card;
}

function openItemDetail(item, category) {
  renderResponse({
    state: "completed",
    summary: `Details for ${item.title}.`,
    components: [{ type: "item_detail", data: { item, category } }],
  });
  setDialog({ icon: "ready", message: `Here are the details for ${item.title}. You can add a picture here too.` });
}

function categoryList(categories) {
  if (!Array.isArray(categories)) categories = categories?.categories || [];
  const card = document.createElement("article");
  card.className = "card";
  card.innerHTML = `<h2>Categories</h2>`;
  const list = document.createElement("ul");
  list.className = "category-list";
  for (const category of categories || []) {
    const li = document.createElement("li");
    const button=document.createElement("button");button.type="button";button.className="category-browse-button";button.textContent=category.name;
    button.addEventListener("click",()=>showCategoryItems(category));
    const detail=document.createElement("span");detail.textContent=`${(category.attributes||[]).length} attributes`;
    li.append(button,detail);
    list.append(li);
  }
  card.append(list);
  return card;
}

async function showBrowseCategories() {
  if(activeController)return;
  const generation=++requestGeneration;
  try {
    const response=await api("/api/categories?limit=100");
    if(generation!==requestGeneration)return;
    renderResponse({state:"completed",summary:"Browse your inventory by collection.",components:[{type:"category_list",data:response.categories||[]}]});
  } catch(error) {renderError(error);}
}

async function showCategoryItems(category,offset=0,append=false) {
  if(activeController)return;
  const generation=++requestGeneration;
  try {
    const response=await api(`/api/items?category_id=${encodeURIComponent(category.id)}&limit=50&offset=${offset}`);
    if(generation!==requestGeneration)return;
    const previous=(currentResponse?.components||[]).find(c=>c.type==="item_list")?.data?.items||[];
    const items=append?[...previous,...(response.items||[])]:response.items||[];
    renderResponse({state:"completed",summary:`${category.name}: ${items.length} items shown.`,components:[{type:"item_list",data:{category,items,offset:offset+(response.items||[]).length,has_more:(response.items||[]).length===50}}]});
  } catch(error) {renderError(error);}
}

function attributeDefinitionTable(attributes, title) {
  const section = document.createElement("section");
  section.className = "data-section";
  section.innerHTML = `<h3>${escapeHTML(title)}</h3>`;
  if (!attributes.length) {
    section.insertAdjacentHTML("beforeend", `<p class="summary">No attributes.</p>`);
    return section;
  }
  const table = document.createElement("table");
  table.innerHTML = "<thead><tr><th>Attribute</th><th>Type</th><th>Options</th><th>Required</th></tr></thead>";
  const body = document.createElement("tbody");
  for (const attribute of attributes) {
    const row = document.createElement("tr");
    const options = attribute.data_type === "enum" ? enumOptions(attribute).join(", ") : "—";
    row.innerHTML = `<td>${escapeHTML(attribute.label || prettyKey(attribute.key))}</td><td>${escapeHTML(fieldTypeLabel(attribute.data_type))}</td><td>${escapeHTML(options)}</td><td>${attribute.required ? "Yes" : "No"}</td>`;
    body.append(row);
  }
  table.append(body);
  section.append(table);
  return section;
}

function attributeValueTable(values, title, previousValues = null) {
  const section = document.createElement("section");
  section.className = "data-section";
  section.innerHTML = `<h3>${escapeHTML(title)}</h3>`;
  const entries = Object.entries(values || {});
  if (!entries.length) {
    section.insertAdjacentHTML("beforeend", `<p class="summary">No details were provided.</p>`);
    return section;
  }
  const table = document.createElement("table");
  const showChanges = previousValues !== null;
  table.innerHTML = showChanges
    ? "<thead><tr><th>Attribute</th><th>Current</th><th>Proposed</th><th>Change</th></tr></thead>"
    : "<thead><tr><th>Attribute</th><th>Value</th></tr></thead>";
  const body = document.createElement("tbody");
  for (const [key, value] of entries) {
    const row = document.createElement("tr");
    if (showChanges) {
      const previous = previousValues[key];
      const changed = !Object.prototype.hasOwnProperty.call(previousValues, key) || !sameValue(previous, value);
      if (changed) row.className = "changed-row";
      row.innerHTML = `<td>${escapeHTML(prettyKey(key))}</td><td>${escapeHTML(formatValue(previous))}</td><td>${escapeHTML(formatValue(value))}</td><td>${changed ? '<span class="change-badge">Will update</span>' : '<span class="unchanged">Unchanged</span>'}</td>`;
    } else {
      row.innerHTML = `<td>${escapeHTML(prettyKey(key))}</td><td>${escapeHTML(formatValue(value))}</td>`;
    }
    body.append(row);
  }
  table.append(body);
  section.append(table);
  return section;
}

function itemsTable(items, category, title) {
  const section = document.createElement("section");
  section.className = "data-section";
  section.innerHTML = `<h3>${escapeHTML(title)}</h3>`;
  const attributes = category.attributes || [];
  const table = document.createElement("table");
  const head = document.createElement("thead");
  head.innerHTML = `<tr><th>Picture</th><th>Item</th><th>Quantity</th>${attributes.map((attribute) => `<th>${escapeHTML(attribute.label || prettyKey(attribute.key))}</th>`).join("")}</tr>`;
  const body = document.createElement("tbody");
  for (const item of items) {
    const values = item.attributes || {};
    const row = document.createElement("tr");
    row.append(itemPictureCell(item, category));
    const titleCell = document.createElement("td");
    const titleButton = document.createElement("button");
    titleButton.type = "button";
    titleButton.className = "item-title-button";
    titleButton.textContent = item.title;
    titleButton.addEventListener("click", () => openItemDetail(item, category));
    titleCell.append(titleButton);
    row.append(titleCell);
    row.insertAdjacentHTML("beforeend", `<td>${escapeHTML(String(item.quantity))}</td>${attributes.map((attribute) => `<td>${escapeHTML(formatValue(values[attribute.key]))}</td>`).join("")}`);
    body.append(row);
  }
  table.append(head, body);
  section.append(table);
  return section;
}

function itemPictureCell(item, category) {
  const cell = document.createElement("td");
  cell.className = "picture-cell";
  const picture = (item.images || [])[0];
  if (picture) {
    const button = document.createElement("button");
    button.className = "thumbnail-button";
    button.type = "button";
    button.title = `View picture for ${item.title}`;
    const thumbnailURL = picture.thumbnail_url || `/api/images/${picture.id}/thumbnail`;
    const originalURL = picture.original_url || `/api/images/${picture.id}/original`;
    button.innerHTML = `<img src="${escapeHTML(thumbnailURL)}" alt="Picture of ${escapeHTML(item.title)}">`;
    button.addEventListener("click", () => showItemImage(item.title, originalURL));
    cell.append(button);
    return cell;
  }

  const button = document.createElement("button");
  button.type = "button";
  button.className = "upload-picture secondary";
  button.textContent = "Add";
  button.setAttribute("aria-label", `Add picture for ${item.title}`);
  button.addEventListener("click", () => openItemDetail(item, category));
  cell.append(button);
  return cell;
}

function itemPicturePanel(item) {
  const section = document.createElement("section");
  section.className = "item-picture-panel";
  section.innerHTML = "<h3>Picture</h3>";
  const picture = (item.images || [])[0];
  if (picture) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "item-picture-button";
    const thumbnailURL = picture.thumbnail_url || `/api/images/${picture.id}/thumbnail`;
    const originalURL = picture.original_url || `/api/images/${picture.id}/original`;
    button.innerHTML = `<img src="${escapeHTML(thumbnailURL)}" alt="Picture of ${escapeHTML(item.title)}"><span>View full size</span>`;
    button.addEventListener("click", () => showItemImage(item.title, originalURL));
    section.append(button);
    return section;
  }

  const input = document.createElement("input");
  input.type = "file";
  input.accept = "image/jpeg,image/png,image/gif";
  input.className = "visually-hidden";
  input.setAttribute("aria-label", `Choose a picture for ${item.title}`);
  const dropZone = document.createElement("div");
  dropZone.className = "photo-dropzone";
  dropZone.tabIndex = 0;
  dropZone.setAttribute("role", "button");
  dropZone.setAttribute("aria-label", `Add a picture for ${item.title}`);
  dropZone.innerHTML = `<svg class="upload-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 16V4m0 0-4 4m4-4 4 4M5 14v4a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-4"/></svg><strong>Drop a picture here</strong><span>or <span class="photo-select-action">choose a file</span></span><small>JPEG, PNG, or GIF · up to 12 MB</small>`;

  const choose = () => { if (!dropZone.classList.contains("uploading")) input.click(); };
  dropZone.addEventListener("click", choose);
  dropZone.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      choose();
    }
  });
  for (const eventName of ["dragenter", "dragover"]) {
    dropZone.addEventListener(eventName, (event) => {
      event.preventDefault();
      if (!dropZone.classList.contains("uploading")) dropZone.classList.add("dragging");
    });
  }
  for (const eventName of ["dragleave", "drop"]) {
    dropZone.addEventListener(eventName, (event) => {
      event.preventDefault();
      dropZone.classList.remove("dragging");
    });
  }
  dropZone.addEventListener("drop", (event) => {
    const file = event.dataTransfer?.files?.[0];
    if (file) uploadItemPicture(item, file, section, dropZone);
  });
  input.addEventListener("change", () => {
    const file = input.files?.[0];
    if (file) uploadItemPicture(item, file, section, dropZone);
  });
  section.append(dropZone, input);
  return section;
}

async function uploadItemPicture(item, file, section, dropZone) {
  if (!["image/jpeg", "image/png", "image/gif"].includes(file.type)) {
    setDialog({ icon: "error", message: "Choose a JPEG, PNG, or GIF image." });
    return;
  }
  if (file.size > 12 * 1024 * 1024) {
    setDialog({ icon: "error", message: "That picture is larger than the 12 MB upload limit." });
    return;
  }
  dropZone.classList.add("uploading");
  dropZone.setAttribute("aria-busy", "true");
  dropZone.innerHTML = `<span class="upload-spinner" aria-hidden="true"></span><strong>Uploading ${escapeHTML(file.name)}</strong><span>Please keep this page open</span>`;
  try {
    const body = new FormData();
    body.append("image", file);
    const image = await api(`/api/items/${item.id}/images`, { method: "POST", body });
    item.images = [image];
    section.replaceWith(itemPicturePanel(item));
    setDialog({ icon: "celebrating", message: `The picture for ${item.title} is uploaded and ready to view.` });
  } catch (error) {
    section.replaceWith(itemPicturePanel(item));
    setDialog({ icon: "error", message: `I couldn't upload that picture: ${error.message}` });
  }
}

function showItemImage(title, url) {
  imageModalTitle.textContent = title;
  imageModalContent.src = url;
  imageModalContent.alt = `Full-size picture of ${title}`;
  imageModal.showModal();
}

function textCard(text) {
  const card = document.createElement("article");
  card.className = "card";
  card.textContent = text;
  return card;
}

function appendDetail(list, label, value) {
  if (value === undefined || value === null || value === "") return;
  const term = document.createElement("dt");
  term.textContent = label;
  const detail = document.createElement("dd");
  detail.textContent = String(value);
  list.append(term, detail);
}

function renderStopped() {
  setDialog({ icon: "ready", message: "Stopped—your request is back in the composer whenever you’re ready." });
  input.value = activeMessage;
}

function setWorking(message) {
  input.disabled = true;
  sendButton.disabled = true;
  input.hidden = true;
  submittedMessage.textContent = message;
  submittedMessage.hidden = false;
  composerHint.textContent = "Remy is working on this request";
  sendButton.hidden = true;
  stopRequest.hidden = false;
  stopRequest.disabled = false;
}

async function fetchDialog(phase, message, context, signal) {
  try {
    return await api("/api/remy/dialog", {
      method: "POST",
      body: JSON.stringify({ phase, message, context }),
      signal,
    });
  } catch (error) {
    if (error.name === "AbortError") throw error;
    return null;
  }
}

async function waitForWorkingDialog(shownAt, signal) {
  if (!shownAt) return;
  const remaining = minimumWorkingDialogMs - (Date.now() - shownAt);
  if (remaining <= 0) return;
  await abortableDelay(remaining, signal);
}

function abortableDelay(milliseconds, signal) {
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(() => {
      signal?.removeEventListener("abort", abort);
      resolve();
    }, milliseconds);
    const abort = () => {
      window.clearTimeout(timer);
      const error = new Error("Request stopped");
      error.name = "AbortError";
      reject(error);
    };
    if (signal?.aborted) abort();
    else signal?.addEventListener("abort", abort, { once: true });
  });
}

function completionFallback(response) {
  if (response?.state === "error") return { icon: "error", message: "I hit a snag while handling that request." };
  if (!(response?.components || []).length) return { icon: "ready", message: "I’m best at inventory—try asking me to add, update, find, or organize an item." };
  return { icon: response.state === "proposing" ? "cataloging" : "celebrating", message: "I’ve put your inventory details in view and ready to review." };
}

function setDialog(response) {
  const icon = ["ready", "thinking", "searching", "cataloging", "celebrating", "error"].includes(response.icon) ? response.icon : "ready";
  dialog.textContent = truncateDialog(response.message || "Hi, I'm Remy. How can I help you manage your inventory.");
  setRemyImage(icon);
}

function truncateDialog(message) {
  const characters = Array.from(String(message).trim().replace(/\s+/g, " "));
  return characters.length <= 140 ? characters.join("") : `${characters.slice(0, 139).join("").trim()}…`;
}

function renderEmpty() {
  stage.innerHTML = `<div class="empty"><h2>Your inventory workspace</h2><p>Inventory details, search results, and proposals will appear here.</p></div>`;
}

function clearWorking() {
  activeController = null;
  input.disabled = false;
  sendButton.disabled = false;
  submittedMessage.hidden = true;
  submittedMessage.textContent = "";
  input.hidden = false;
  composerHint.textContent = "Enter to send · Shift+Enter for a new line";
  stopRequest.hidden = true;
  sendButton.hidden = false;
  input.focus();
}

function setRemyImage(stateName) {
  const labels = { ready: "Remy is ready", thinking: "Remy is thinking", searching: "Remy is searching the inventory", cataloging: "Remy is preparing an inventory proposal", celebrating: "Remy is ready for the next request", error: "Remy needs help" };
  remyAvatar.src = `/static/remy-${stateName}.svg`;
  remyAvatar.alt = labels[stateName] || labels.ready;
}

function operationLabel(operation) { return ({ create: "Add", update: "Update", delete: "Delete", quantity_adjust: "Adjust quantity" })[operation] || operation; }
function signed(value) { return Number(value) > 0 ? `+${value}` : String(value); }
function prettyKey(key) { return String(key || "").replaceAll("_", " ").replace(/\b\w/g, (letter) => letter.toUpperCase()); }
function formatValue(value) { if (value === undefined || value === null || value === "") return "—"; if (typeof value === "boolean") return value ? "Yes" : "No"; return String(value); }
function sameValue(left, right) { return JSON.stringify(left) === JSON.stringify(right); }
function escapeHTML(value) { return String(value).replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "&#039;"); }
function enumOptions(attribute) {
  try {
    const config = typeof attribute.config === "string" ? JSON.parse(attribute.config || "{}") : (attribute.config || {});
    return Array.isArray(config.options) ? config.options : [];
  } catch (_) {
    return [];
  }
}
function fieldTypeLabel(type) { return ({ text: "Text", number: "Number", boolean: "Yes / no", date: "Date", enum: "Enumeration" })[type] || prettyKey(type || "text"); }
