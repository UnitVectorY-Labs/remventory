const form = document.querySelector("#remy-form");
const input = document.querySelector("#message");
const stage = document.querySelector("#stage");
const state = document.querySelector("#state");

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const message = input.value.trim();
  if (!message) return;
  input.value = "";
  await askRemy(message);
});

async function askRemy(message) {
  setState("Thinking");
  stage.innerHTML = "";
  try {
    const response = await api("/api/remy/request", {
      method: "POST",
      body: JSON.stringify({ message }),
    });
    renderResponse(response);
  } catch (error) {
    renderError(error);
  }
}

async function api(path, options = {}) {
  const response = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(body.error || response.statusText);
  }
  return body;
}

function renderResponse(response) {
  setState(labelState(response.state));
  stage.innerHTML = "";
  if (response.summary) {
    const summary = document.createElement("p");
    summary.className = "summary";
    summary.textContent = response.summary;
    stage.append(summary);
  }
  for (const component of response.components || []) {
    stage.append(renderComponent(component));
  }
}

function renderComponent(component) {
  switch (component.type) {
    case "category_proposal":
      return proposalCard(component.data, "Category Proposal");
    case "item_proposal":
      return proposalCard(component.data, "Item Proposal");
    case "item_list":
      return itemList(component.data);
    case "category_list":
      return categoryList(component.data);
    case "text":
      return textCard(component.data);
    default:
      return jsonCard(component.type, component.data);
  }
}

function proposalCard(proposal, title) {
  const card = document.createElement("article");
  card.className = "card";
  const payload = proposal.proposed_payload || {};
  card.innerHTML = `
    <h2>${escapeHTML(title)}</h2>
    <div class="grid">
      ${field("Status", proposal.status)}
      ${field("Type", proposal.type)}
      ${field("Name", payload.name || payload.title || "")}
      ${field("Description", payload.description || "")}
    </div>
  `;

  if (Array.isArray(payload.attributes)) {
    const attrs = document.createElement("div");
    attrs.className = "card";
    attrs.innerHTML = `<h3>Attributes</h3>`;
    for (const attr of payload.attributes) {
      attrs.insertAdjacentHTML("beforeend", field(attr.label || attr.key, attr.data_type || "text"));
    }
    card.append(attrs);
  } else if (payload.attributes && typeof payload.attributes === "object") {
    card.append(jsonCard("Attributes", payload.attributes));
  }

  if (proposal.status === "pending") {
    const actions = document.createElement("div");
    actions.className = "actions";
    const approve = document.createElement("button");
    approve.textContent = "Approve";
    approve.addEventListener("click", () => decide(proposal.id, true));
    const reject = document.createElement("button");
    reject.className = "danger";
    reject.textContent = "Reject";
    reject.addEventListener("click", () => decide(proposal.id, false));
    actions.append(approve, reject);
    card.append(actions);
  }

  return card;
}

async function decide(id, approve) {
  setState("Confirming");
  try {
    const proposal = await api(`/api/proposals/${id}/decision`, {
      method: "POST",
      body: JSON.stringify({ approve, reason: approve ? "" : "Rejected in web UI" }),
    });
    renderResponse({
      state: "completed",
      summary: approve ? "Approved and committed." : "Rejected.",
      components: [{ type: proposal.type === "category_create" ? "category_proposal" : "item_proposal", data: proposal }],
    });
  } catch (error) {
    renderError(error);
  }
}

function itemList(data) {
  const card = document.createElement("article");
  card.className = "card";
  card.innerHTML = `<h2>${escapeHTML(data.category?.name || "Items")}</h2>`;
  const items = data.items || [];
  if (!items.length) {
    card.insertAdjacentHTML("beforeend", `<p class="summary">No items yet.</p>`);
    return card;
  }
  for (const item of items) {
    const row = document.createElement("div");
    row.className = "field";
    row.innerHTML = `<div class="value"><strong>${escapeHTML(item.title)}</strong> · Quantity ${escapeHTML(String(item.quantity))}</div>`;
    if (item.attributes) {
      row.append(jsonCard("Details", item.attributes));
    }
    card.append(row);
  }
  return card;
}

function categoryList(categories) {
  const card = document.createElement("article");
  card.className = "card";
  card.innerHTML = `<h2>Categories</h2>`;
  for (const category of categories || []) {
    card.insertAdjacentHTML("beforeend", field(category.name, `${(category.attributes || []).length} attributes`));
  }
  return card;
}

function textCard(text) {
  const card = document.createElement("article");
  card.className = "card";
  card.textContent = text;
  return card;
}

function jsonCard(title, data) {
  const card = document.createElement("article");
  card.className = "card";
  card.innerHTML = `<h3>${escapeHTML(title)}</h3><pre>${escapeHTML(JSON.stringify(data, null, 2))}</pre>`;
  return card;
}

function field(label, value) {
  if (value === undefined || value === null || value === "") return "";
  return `<div class="field"><div class="label">${escapeHTML(label)}</div><div class="value">${escapeHTML(String(value))}</div></div>`;
}

function renderError(error) {
  setState("Error");
  stage.innerHTML = "";
  stage.append(textCard(error.message));
}

function setState(value) {
  state.textContent = value;
}

function labelState(value) {
  if (!value) return "Ready";
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}
