# Remy and the collection

Remventory combines a visual collection with Remy, a playful hamster librarian. The always-visible request composer is a central way to find, compare, add, modify, and delete inventory. Remy responds with task-specific cards, details, comparisons, statistics, and approval forms. Direct browsing and entry complement that interface.

## An intentional vocabulary

An item has an internal ID, overlapping tags, optional values for shared fields, and photographs. No name, location, quantity, category, or other metadata value is mandatory. The UI never needs to show the internal item ID. An optional Title value supplies a display label; otherwise the tags or a generic label identify the card alongside its picture.

Fields are defined once in the shared vocabulary, with stable keys, labels, and value types. For example, `title` always means Title and `year` always means a numeric Year. Item forms select existing definitions; typing a new attribute name into an item does not expand the vocabulary.

New definitions enter through a separate reviewed proposal. Remy has the same proposal capability and cannot approve its own definitions. Unknown keys are rejected when an item proposal is created and checked again when it is approved. Existing metadata from earlier versions is preserved and its vocabulary carried into the registry by migration. Earlier category names become tags, their field associations become tag suggestions, and their definitions become creation templates.

Supported field types are text, number, boolean, date, enum, and lists of links. A link list contains named references such as:

```json
{"links": [
  {"name": "Wikipedia", "url": "https://en.wikipedia.org/wiki/Wii"},
  {"name": "Manufacturer", "url": "https://www.nintendo.com/"}
]}
```

The names of these references are list values, not separate field definitions. Location and Quantity are available as ordinary optional fields. They receive no dedicated controls or badges in the collection.

## Tags suggest, rather than constrain

Tags overlap without a hierarchy: a console can carry Consoles + Wii and a game Games + Wii. Selecting Wii finds both; selecting Games as well finds their intersection.

Tags may have explicitly approved suggested fields. Games might suggest Title, Year, Publisher, Platform, and Links. These suggestions help build an entry form and give Remy context. They do not require values or restrict which other approved fields can be associated with an item.

The **Fields & tag suggestions** screen defines new fields and edits these associations. It separates vocabulary maintenance from entering item values, with review before approval.

## Templates are creation shortcuts

A creation template collects shared fields, default values, and default tags. It copies that starting point into a new draft. Values entered by the person override defaults, and the draft can add or remove fields and tags immediately.

An approved item has no continuing template membership. Changing or deleting a template does not change existing items. The template editor selects existing field definitions; new definitions must be approved separately. Remy receives the approved field vocabulary, tag suggestions, and templates as request context and can propose item creation using a template.

## Photos and cataloging

1. Open **Add item**, optionally starting from a template.
2. Choose multiple pictures or take a picture. Choose which should be primary.
3. Optionally ask Remy to read a selected picture. Identification can suggest likely matches, but must explain uncertainty. Only registered field keys become editable suggestions; unregistered keys are mentioned for separate approval.
4. Add tags, choose defined fields, and enter values. Remove any field that is not useful.
5. Review and explicitly approve the item, photo uploads, and selected primary image.

The original photo stays unmodified in S3-compatible storage. Collection cards use generated 320-pixel square JPEG thumbnails of the selected primary photo. Full-resolution photos are available from the detail view. Identification uses a separate uncropped image bounded to 1600 pixels on its longest side. Re-uploading identical bytes to an item returns its existing image.

The first uploaded photo becomes primary automatically. Changing the primary photo uses an item proposal and approval, with server validation that the selected image belongs to the item. The database enforces a single primary image per item, and both detail and collection queries put it first.

The current draft lives in the open browser dialog. It does not survive closing the dialog or reloading. Item values commit before uploads; failed photo uploads or primary selection can be retried without recreating the item. Photos do not participate in the inventory proposal transaction.

## Remy and models

The visible composer accepts an outcome in natural language. Remy chooses useful typed interface components backed by stored evidence, rather than producing a serial chat log as the primary output. All proposed inventory and vocabulary mutations await human approval.

The text and photo models have independent configuration, including optional separate vision provider credentials. Models are invoked on demand. Canceling photo identification stops waiting; inference cancellation and unloading depend on the provider. Browsing, deterministic search, and manual cataloging work without a model.

AI search uses inventory tools over tags and registered values. Embeddings and image similarity are not implemented. New attribute definitions, tag-field associations, templates, and primary-photo changes are exposed through MCP alongside the HTTP interface.

## Further relationships

The current graph has items associated with tags and field values, tags suggesting fields, and templates providing starting fields, values, and tags. It does not yet include tag aliases, broader-tag edges, or item-to-item relationships. Those can be represented by explicit tables in Postgres when their review workflows are added; this experience does not require a separate graph database.
