const form = document.querySelector("#remy-form");
const input = document.querySelector("#message");
const sendButton = document.querySelector("#send-request");
const newChat = document.querySelector("#new-chat");
const stage = document.querySelector("#stage");
const state = document.querySelector("#state");
const processing = document.querySelector("#processing");
const processingMessage = document.querySelector("#processing-message");
const stopRequest = document.querySelector("#stop-request");
const configurationWarning = document.querySelector("#configuration-warning");
const remyAvatar = document.querySelector("#remy-avatar");
let activeController = null;
let activeMessage = "";
let currentResponse = null;

showConfigurationStatus();

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
  if (activeController) activeController.abort();
  currentResponse = null;
  activeMessage = "";
  stage.innerHTML = `<div class="empty"><h2>What are we inventorying?</h2><p>Start a fresh conversation with Remy. Nothing shown here has been changed.</p></div>`;
  setState("Ready");
  setRemyImage("ready");
  input.value = "";
  input.focus();
});

stopRequest.addEventListener("click", () => {
  if (!activeController) return;
  setState("Stopping");
  stopRequest.disabled = true;
  processingMessage.textContent = "Stopping Remy’s request";
  activeController.abort();
});

const initialMessage = new URLSearchParams(window.location.search).get("message");
if (initialMessage) {
  input.value = initialMessage;
  form.requestSubmit();
}

async function askRemy(message) {
  activeController = new AbortController();
  activeMessage = message;
  const context = visibleContext(currentResponse);
  setWorking(message);
  try {
    const response = await api("/api/remy/request", {
      method: "POST",
      body: JSON.stringify({ message, context }),
      signal: activeController.signal,
    });
    renderResponse(response);
  } catch (error) {
    if (error.name === "AbortError") {
      renderStopped();
    } else {
      renderError(error);
    }
  } finally {
    clearWorking();
  }
}

async function api(path, options = {}) {
  const response = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error || response.statusText);
  return body;
}

async function showConfigurationStatus() {
  try {
    const status = await api("/api/config");
    if (!status.config?.model_configured) {
      configurationWarning.hidden = false;
      configurationWarning.textContent = "Remy needs OPENAI_MAIN_MODEL and OPENAI_THINKING_MODEL before requests can use the full agent workflow.";
    }
  } catch (_) {
    // Submission errors remain close to the request surface.
  }
}

function renderResponse(response) {
  currentResponse = response;
  setState(response.state === "proposing" ? "Review" : "Ready");
  setRemyImage(response.state === "proposing" ? "cataloging" : "celebrating");
  stage.innerHTML = "";
  if (response.request_summary) {
    const requestSummary = document.createElement("p");
    requestSummary.className = "request-summary";
    requestSummary.textContent = response.request_summary;
    stage.append(requestSummary);
  }
  if (response.summary) {
    const summary = document.createElement("p");
    summary.className = "summary";
    summary.textContent = response.summary;
    stage.append(summary);
  }
  for (const component of response.components || []) stage.append(renderComponent(component));
}

function visibleContext(response) {
  if (!response) return null;
  return { state: response.state, summary: response.summary, request_summary: response.request_summary, components: response.components || [] };
}

function renderComponent(component) {
  switch (component.type) {
    case "category_proposal": return proposalCard(component.data, "Category change");
    case "item_proposal": return proposalCard(component.data, "Item change");
    case "category_definition": return categoryDefinition(component.data);
    case "item_list": return itemList(component.data);
    case "query_result": return queryResult(component.data);
    case "category_list": return categoryList(component.data);
    case "text": return textCard(component.data);
    default: return textCard("Remy returned a result that this version cannot display yet.");
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
  setState("Saving");
  try {
    const proposal = await api(`/api/proposals/${id}/decision`, {
      method: "POST",
      body: JSON.stringify({ approve, reason: "" }),
    });
    renderResponse({
      state: "completed",
      summary: approve ? "Changes approved and saved." : "Proposal rejected. No inventory data was changed.",
      components: [{ type: proposal.type === "category_create" ? "category_proposal" : "item_proposal", data: proposal }],
    });
  } catch (error) {
    renderError(error);
  }
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
  card.innerHTML = `<div class="card-heading"><div><p class="eyebrow">Inventory check</p><h2>${escapeHTML(data.category?.name || "Inventory")}</h2></div></div><p class="summary">${escapeHTML(data.summary || "")}</p>`;
  const matches = data.matches || [];
  if (matches.length) card.append(itemsTable(matches, data.category || {}, "Matching items"));
  return card;
}

function itemList(data) {
  const card = document.createElement("article");
  card.className = "card";
  card.innerHTML = `<div class="card-heading"><div><p class="eyebrow">Inventory</p><h2>${escapeHTML(data.category?.name || "Items")}</h2></div></div>`;
  const items = data.items || [];
  if (!items.length) card.insertAdjacentHTML("beforeend", `<p class="summary">No items yet.</p>`);
  else card.append(itemsTable(items, data.category || {}, "Items"));
  return card;
}

function categoryList(categories) {
  const card = document.createElement("article");
  card.className = "card";
  card.innerHTML = `<h2>Categories</h2>`;
  const list = document.createElement("ul");
  list.className = "category-list";
  for (const category of categories || []) {
    const li = document.createElement("li");
    li.innerHTML = `<strong>${escapeHTML(category.name)}</strong><span>${escapeHTML(String((category.attributes || []).length))} attributes</span>`;
    list.append(li);
  }
  card.append(list);
  return card;
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
  table.innerHTML = "<thead><tr><th>Attribute</th><th>Type</th><th>Required</th></tr></thead>";
  const body = document.createElement("tbody");
  for (const attribute of attributes) {
    const row = document.createElement("tr");
    row.innerHTML = `<td>${escapeHTML(attribute.label || prettyKey(attribute.key))}</td><td>${escapeHTML(attribute.data_type || "text")}</td><td>${attribute.required ? "Yes" : "No"}</td>`;
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
  head.innerHTML = `<tr><th>Item</th><th>Quantity</th>${attributes.map((attribute) => `<th>${escapeHTML(attribute.label || prettyKey(attribute.key))}</th>`).join("")}</tr>`;
  const body = document.createElement("tbody");
  for (const item of items) {
    const values = item.attributes || {};
    const row = document.createElement("tr");
    row.innerHTML = `<td>${escapeHTML(item.title)}</td><td>${escapeHTML(String(item.quantity))}</td>${attributes.map((attribute) => `<td>${escapeHTML(formatValue(values[attribute.key]))}</td>`).join("")}`;
    body.append(row);
  }
  table.append(head, body);
  section.append(table);
  return section;
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

function renderError(error) {
  setState("Needs attention");
  setRemyImage("error");
  stage.innerHTML = "";
  stage.append(textCard(error.message));
}

function renderStopped() {
  setState("Ready");
  setRemyImage("ready");
  input.value = activeMessage;
  processingMessage.textContent = "Request stopped. Your message is back in the composer.";
}

function setState(value) { state.textContent = value; }

function setWorking(message) {
  setState("Working");
  const lower = message.toLowerCase();
  setRemyImage(lower.includes("show") || lower.includes("list") || lower.includes("have") ? "searching" : "thinking");
  input.disabled = true;
  input.classList.add("submitted");
  sendButton.disabled = true;
  sendButton.textContent = "Sent";
  processing.hidden = false;
  stopRequest.disabled = false;
  processingMessage.textContent = `Remy is working on: ${message}`;
}

function clearWorking() {
  activeController = null;
  input.disabled = false;
  input.classList.remove("submitted");
  sendButton.disabled = false;
  sendButton.textContent = "Send";
  processing.hidden = true;
  input.focus();
}

function setRemyImage(stateName) {
  const labels = { ready: "Remy the hamster librarian is ready", thinking: "Remy the hamster librarian is thinking", searching: "Remy the hamster librarian is searching the catalog", cataloging: "Remy the hamster librarian is preparing an inventory proposal", celebrating: "Remy the hamster librarian is ready for the next request", error: "Remy the hamster librarian needs help" };
  remyAvatar.src = `/static/remy-${stateName}.svg`;
  remyAvatar.alt = labels[stateName] || labels.ready;
}

function operationLabel(operation) { return ({ create: "Add", update: "Update", delete: "Delete", quantity_adjust: "Adjust quantity" })[operation] || operation; }
function signed(value) { return Number(value) > 0 ? `+${value}` : String(value); }
function prettyKey(key) { return String(key || "").replaceAll("_", " ").replace(/\b\w/g, (letter) => letter.toUpperCase()); }
function formatValue(value) { if (value === undefined || value === null || value === "") return "—"; if (typeof value === "boolean") return value ? "Yes" : "No"; return String(value); }
function sameValue(left, right) { return JSON.stringify(left) === JSON.stringify(right); }
function escapeHTML(value) { return String(value).replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "&#039;"); }
