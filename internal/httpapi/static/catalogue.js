// One item workspace: verified catalogue facts, editable owner fields and references.
function remyBusy(message) {
  setDialog({ icon: "thinking", message });
}
function readableValue(value) {
  return Array.isArray(value)
    ? value
        .map((v) =>
          typeof v === "object"
            ? v.name || v.url || JSON.stringify(v)
            : String(v),
        )
        .join(" · ")
    : typeof value === "object"
      ? JSON.stringify(value)
      : String(value);
}
itemDetail = function (data) {
  const item = data.item || {},
    root = el("article", "curated-item");
  root.append(el("h2", "", item.title || "Your item"));
  const tools = el("div", "item-detail-tools");
  tools.append(
    button("← Collection", () => showLibrary(), "plain-button"),
    button("Edit my fields & photos", () => openCatalog(item), "secondary"),
    button(
      "Manage catalogue & links",
      () => manageCatalogue(item),
      "secondary",
    ),
  );
  root.append(tools);
  const layout = el("div", "match-layout"),
    photo = el("div", "flow-picture"),
    content = el("div");
  if (item.images?.length) {
    const p = item.images.find((p) => p.is_primary) || item.images[0],
      img = el("img");
    img.src = p.original_url || `/api/images/${p.id}/original`;
    img.alt = item.title;
    img.onclick = () => showItemImage(item.title, img.src);
    photo.append(img);
  }
  const catalogue = el("section", "catalogue-facts");
  catalogue.append(
    el("h3", "", "Catalogue facts"),
    el("p", "muted", "Loading the linked records…"),
  );
  const owner = el("section", "owner-fields");
  owner.append(
    el("h3", "", "My fields"),
    el(
      "p",
      "muted",
      "Values you entered. You can keep, edit or remove these; catalogue facts stay with their source.",
    ),
  );
  const dl = el("dl", "identity-facts");
  for (const [key, value] of Object.entries(item.attributes || {})) {
    if (key === "title" || key === "links") continue;
    appendAttributeDetail(dl, key, value);
  }
  if (dl.children.length) owner.append(dl);
  else owner.append(el("p", "muted", "No extra personal fields."));
  const review = button(
    "Ask Remy to tidy my fields",
    () => reviewOwnerFields(item),
    "plain-button",
  );
  owner.append(review);
  if (item.attributes?.links?.length) {
    const refs = el("section", "reference-links");
    refs.append(el("h3", "", "My references"));
    for (const link of item.attributes.links) {
      const a = el("a", "source-link", link.name || link.url);
      if (/^https?:\/\//i.test(link.url)) {
        a.href = link.url;
        a.target = "_blank";
        a.rel = "noopener noreferrer";
      }
      refs.append(a);
    }
    owner.append(refs);
  }
  owner.append(el("p", "item-tags", (item.tags || []).join(" · ")));
  content.append(catalogue, owner);
  layout.append(photo, content);
  root.append(layout);
  api(`/api/items/${item.id}/cards`)
    .then((data) => {
      catalogue.replaceChildren(el("h3", "", "Catalogue facts"));
      if(root.isConnected)setDialog({icon:"ready",message:"Here are the linked facts and your own fields. I can help you keep them tidy."});
      const cards = data.cards || [];
      if (!cards.length)
        catalogue.append(
          el("p", "", "No catalogue record is linked yet."),
          button("Let Remy find it", () => researchItem(item)),
          button(
            "Choose a record or paste a URL",
            () => manageCatalogue(item),
            "secondary",
          ),
        );
      for (const c of cards) catalogue.append(factCard(c, true));
      if (cards.length)
        catalogue.append(
          button(
            "Choose the facts to show",
            () => chooseCatalogueFields(item),
            "plain-button",
          ),
        );
    })
    .catch((e) => flowError(catalogue, e));
  return root;
};
async function refreshOwnedItem(id) {
  const item = await api(`/api/items/${id}`);
  openItemDetail(item.item || item, {});
}
function catalogueConfirmation(item, data, restore) {
  catalogModal.replaceChildren(dialogHeader("Confirm this change"));
  const body = el("div", "catalogue-confirm");
  if (data.card) body.append(factCard(data.card));
  const p = data.proposal,
    payload = p.proposed_payload;
  body.append(
    el(
      "p",
      "",
      data.reference
        ? `${data.reference.operation === "reference_remove" ? "Remove" : "Save"} “${data.reference.name}” as a reference link. Remy will not treat it as verified catalogue metadata.`
        : payload.operation === "unlink"
          ? `Unlink “${payload.label}” from this item? The cached record and your personal fields are kept.`
          : `Link “${payload.label}” to this item? Your personal fields stay unchanged.`,
    ),
  );
  const status = el("p", "flow-status");
  const confirm = button("Confirm", async () => {
    confirm.disabled = true;
    try {
      await api(`/api/proposals/${p.id}/decision`, {
        method: "POST",
        body: JSON.stringify({ approve: true }),
      });
      catalogModal.close();
      setDialog({
        icon: "celebrating",
        message:
          "Done. Your catalogue links and your own fields remain separate.",
      });
      await refreshOwnedItem(item.id);
    } catch (e) {
      flowError(status, e);
      confirm.disabled = false;
    }
  });
  const back = button(
    "Cancel",
    async () => {
      try {
        await api(`/api/proposals/${p.id}/decision`, {
          method: "POST",
          body: JSON.stringify({ approve: false }),
        });
        restore();
      } catch (e) {
        flowError(status, e);
      }
    },
    "secondary",
  );
  body.append(status, back, confirm);
  catalogModal.append(body);
}
async function manageCatalogue(item) {
  const render = async () => {
    catalogModal.replaceChildren(dialogHeader("Catalogue & reference links"));
    const body = el("div", "catalogue-manager"),
      status = el("p", "flow-status");
    status.setAttribute("role", "status");
    catalogModal.append(body, status);
    if (!catalogModal.open) catalogModal.showModal();
    body.append(
      el(
        "p",
        "muted",
        "Choose a catalogue record yourself, ask Remy for suggestions, or save an ordinary reference URL.",
      ),
    );
    const current = el("section");
    current.append(el("h3", "", "Linked records"));
    body.append(current);
    try {
      const data = await api(`/api/items/${item.id}/canonical`);
      for (const link of data.links || []) {
        const row = el("div", "catalogue-link-row");
        row.append(
          el("span", "", `${link.label} · ${link.provider} · ${link.relation}`),
          button(
            "Unlink",
            () =>
              prepare({
                operation: "unlink",
                entity_id: link.id,
                relation: link.relation,
              }),
            "plain-button",
          ),
        );
        current.append(row);
      }
      if (!data.links?.length)
        current.append(el("p", "muted", "No linked records."));
    } catch (e) {
      flowError(status, e);
    }
    if (item.attributes?.links?.length) {
      const refs = el("section");
      refs.append(el("h3", "", "Reference links"));
      for (const link of item.attributes.links) {
        const row = el("div", "catalogue-link-row");
        row.append(
          el("span", "", link.name || link.url),
          button(
            "Remove reference",
            () =>
              prepare({
                operation: "reference_remove",
                url: link.url,
                name: link.name,
              }),
            "plain-button",
          ),
        );
        refs.append(row);
      }
      body.append(refs);
    }
    const form = el("form", "manual-url-form"),
      kind = el("select");
    kind.append(
      new Option("Catalogue record", "link"),
      new Option("Reference link only", "reference"),
    );
    const url = field("", "url");
    url.required = true;
    url.placeholder = "https://…";
    const name = field();
    name.placeholder = "Link name";
    name.hidden = true;
    kind.onchange = () => {
      name.hidden = kind.value !== "reference";
      name.required = kind.value === "reference";
    };
    const relation = el("select");
    for (const [v, l] of [
      ["identified_as", "Identifies this item"],
      ["contains", "A work contained in this item"],
      ["associated_with", "Related to this item"],
    ])
      relation.append(new Option(l, v));
    const add = el("button", "", "Review link");
    add.type = "submit";
    form.append(
      el("h3", "", "Paste a URL"),
      kind,
      labeled("URL", url),
      name,
      labeled("Relationship", relation),
      add,
    );
    form.onsubmit = (e) => {
      e.preventDefault();
      prepare({
        operation: kind.value,
        url: url.value,
        name: name.value,
        relation: relation.value,
      });
    };
    body.append(form);
    const search = el("form", "catalogue-search"),
      provider = el("select");
    for (const [v, l] of [
      ["amiibo", "Amiibo"],
      ["musicbrainz", "MusicBrainz releases"],
      ["wikidata", "Wikidata"],
    ])
      provider.append(new Option(l, v));
    const query = field();
    query.required = true;
    query.placeholder = "Character, title or model";
    const find = el("button", "secondary", "Search catalogue");
    find.type = "submit";
    const results = el("div", "search-options");
    search.append(
      el("h3", "", "Or search a catalogue directly"),
      provider,
      query,
      find,
    );
    body.append(
      search,
      results,
      button(
        "Ask Remy to identify it",
        () => {
          catalogModal.close();
          researchItem(item);
        },
        "plain-button",
      ),
    );
    search.onsubmit = async (e) => {
      e.preventDefault();
      find.disabled = true;
      remyBusy("I’m searching that catalogue for you.");
      results.textContent = "Searching…";
      try {
        const data = await api(
          `/api/authorities/search?provider=${provider.value}&q=${encodeURIComponent(query.value)}`,
        );
        results.replaceChildren();
        for (const c of data.candidates || []) {
          const choose = button(
            "",
            () =>
              prepare({
                operation: "link",
                ref: { provider: c.provider, kind: c.kind, id: c.id },
                relation: relation.value,
              }),
            "search-option",
          );
          choose.append(
            el("strong", "", c.label),
            el("small", "", c.description),
          );
          results.append(choose);
        }
        if (!data.candidates?.length)
          results.textContent =
            "No results. You can paste a catalogue URL or save a reference link instead.";
        setDialog({
          icon: "ready",
          message: "Choose the edition or version that matches your item.",
        });
      } catch (error) {
        flowError(results, error);
      } finally {
        find.disabled = false;
      }
    };
    let preparing = false;
    async function prepare(payload) {
      if (preparing) return;
      preparing = true;
      status.textContent = "Remy is preparing a review…";
      remyBusy(
        "I’ll validate that link and show you the change before saving.",
      );
      try {
        const data = await api(`/api/items/${item.id}/catalogue/proposal`, {
          method: "POST",
          body: JSON.stringify(payload),
        });
        catalogueConfirmation(item, data, render);
      } catch (error) {
        flowError(status, error);
      } finally {
        preparing = false;
      }
    }
  };
  await render();
}
async function reviewOwnerFields(item) {
  catalogModal.replaceChildren(dialogHeader("Review my fields"));
  const body = el("div", "field-curation"),
    status = el(
      "p",
      "flow-status",
      "Remy is comparing your fields with the linked catalogue facts…",
    );
  body.append(status);
  catalogModal.append(body);
  catalogModal.showModal();
  remyBusy(
    "I’ll look for descriptive fields that the catalogue already covers. You choose what to remove.",
  );
  try {
    const data = await api(`/api/items/${item.id}/curate`, {
      method: "POST",
      body: "{}",
    });
    status.textContent = data.suggestions.length
      ? "These fields may be redundant. Check each suggestion before removing anything."
      : "I didn’t find fields I can confidently call redundant. You can still edit your fields directly.";
    const chosen = new Set();
    for (const suggestion of data.suggestions) {
      const label = el("label", "curation-choice"),
        check = el("input");
      check.type = "checkbox";
      check.onchange = () =>
        check.checked
          ? chosen.add(suggestion.key)
          : chosen.delete(suggestion.key);
      const text = el("span");
      text.append(
        el(
          "strong",
          "",
          `${prettyKey(suggestion.key)}: ${readableValue(data.attributes[suggestion.key])}`,
        ),
        el("small", "", suggestion.reason),
        el(
          "small",
          "",
          `Catalogue: ${suggestion.source_title} · ${suggestion.source_field}: ${suggestion.source_value}`,
        ),
      );
      label.append(check, text);
      body.append(label);
    }
    const remove = button("Review selected removals", async () => {
      if (!chosen.size) {
        status.textContent = "Select the fields you want to remove.";
        return;
      }
      try {
        const p = await api("/api/proposals/item", {
          method: "POST",
          body: JSON.stringify({
            operation: "update",
            item_id: item.id,
            attributes: {},
            previous_attributes: data.attributes,
            remove_fields: [...chosen],
          }),
        });
        body.replaceChildren(
          el(
            "p",
            "",
            `Remove these owner fields: ${[...chosen].map(prettyKey).join(", ")}. Catalogue facts are kept.`,
          ),
        );
        body.append(
          button("Confirm removal", async () => {
            await api(`/api/proposals/${p.id}/decision`, {
              method: "POST",
              body: JSON.stringify({ approve: true }),
            });
            catalogModal.close();
            await refreshOwnedItem(item.id);
          }),
        );
      } catch (error) {
        flowError(status, error);
      }
    });
    if (data.suggestions.length) body.append(remove);
    body.append(
      button(
        "Edit fields myself",
        () => {
          catalogModal.close();
          openCatalog(item);
        },
        "secondary",
      ),
    );
    setDialog({
      icon: "ready",
      message: "I’ve prepared suggestions. Nothing changes until you confirm.",
    });
  } catch (error) {
    flowError(status, error);
    body.append(
      button(
        "Edit fields myself",
        () => {
          catalogModal.close();
          openCatalog(item);
        },
        "secondary",
      ),
    );
  }
}

const catalogueBaseProposalCard = proposalCard;
proposalCard = function (p, title) {
  if (p.type !== "catalogue_change") return catalogueBaseProposalCard(p, title);
  const c = el("article", "card proposal"),
    v = p.proposed_payload;
  c.append(
    el(
      "h2",
      "",
      v.operation === "unlink"
        ? "Unlink catalogue record"
        : "Link catalogue record",
    ),
    el("p", "", `${v.label} · ${v.relation}`),
  );
  const itemName = el("p", "muted", "For your inventory item");
  c.append(itemName);
  api(`/api/items/${v.item_id}`)
    .then((item) => {
      itemName.textContent = `For: ${item.title}`;
    })
    .catch(() => {});
  if (p.status === "pending")
    c.append(
      button("Confirm", () => decide(p.id, true)),
      button("Reject", () => decide(p.id, false), "secondary"),
    );
  else c.append(el("p", "", p.status));
  return c;
};

async function chooseCatalogueFields(item) {
  catalogModal.replaceChildren(dialogHeader("Choose catalogue facts"));
  const body = el("div", "catalogue-manager"),
    status = el("p", "flow-status", "Loading cached facts…");
  body.append(status);
  catalogModal.append(body);
  catalogModal.showModal();
  try {
    const data = await api(`/api/items/${item.id}/catalogue/fields`),
      profile = data.profile;
    if (!profile.id) {
      status.textContent =
        "Add a tag matching an item type to configure which catalogue facts it displays.";
      body.append(
        button(
          "Edit tags",
          () => {
            catalogModal.close();
            openCatalog(item);
          },
          "secondary",
        ),
      );
      return;
    }
    status.textContent = `Choose up to four facts per source for ${profile.name}. This preference applies to items with this type’s tags. Leave all unchecked to let Remy choose.`;
    const selected = {};
    for (const [provider, choices] of Object.entries(data.choices)) {
      const section = el("fieldset", "type-sources");
      section.append(el("legend", "", provider));
      selected[provider] = new Set(
        profile.intake_config?.display_fields?.[provider] || [],
      );
      for (const [path, value] of Object.entries(choices).sort(([a], [b]) =>
        a.localeCompare(b),
      )) {
        const check = field("", "checkbox");
        check.checked = selected[provider].has(path);
        check.onchange = () => {
          if (check.checked && selected[provider].size >= 4) {
            check.checked = false;
            status.textContent = "Choose up to four facts for each source.";
            return;
          }
          check.checked
            ? selected[provider].add(path)
            : selected[provider].delete(path);
        };
        section.append(labeled(`${prettyKey(path)} · ${value}`, check));
      }
      body.append(section);
    }
    body.append(
      button("Review display preference", () => {
        const intake_config = {
          ...profile.intake_config,
          display_fields: {
            ...profile.intake_config?.display_fields,
            ...Object.fromEntries(
              Object.entries(selected).map(([provider, keys]) => [
                provider,
                [...keys],
              ]),
            ),
          },
        };
        reviewDefinition(
          "/api/proposals/category",
          {
            operation: "update",
            category_id: profile.id,
            name: profile.name,
            intake_config,
          },
          "Review catalogue display",
          () => chooseCatalogueFields(item),
        );
      }),
    );
  } catch (e) {
    flowError(status, e);
  }
}
