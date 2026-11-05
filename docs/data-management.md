# Managing an item with Remy

Remy keeps your physical item separate from the catalogue records describing it.
The item view shows **Catalogue facts** with source links and **My fields** with
all your entered values visible. Nothing is moved or deleted just because a
catalogue match was approved.

Use **Manage catalogue & links** to:

- Search a catalogue directly and choose a release or variant without running AI
  identification again.
- Paste a MusicBrainz **release** URL, a Wikidata entity URL, or an AmiiboAPI figure
  URL. Remy validates the identifier through that provider or its verified cache,
  shows the record, then asks you to confirm the association.
- Choose whether a record identifies the item, is contained in it, or is related.
- Unlink a specific association. This preserves the inventory item, photos,
  personal fields, and cached source entity.
- Save any ordinary HTTP/HTTPS URL as a named reference. References are searchable
  owner data; the server does not fetch them, extract metadata, or claim they are
  verified catalogue identities. References can also be removed individually.

Each change has a concrete confirmation step and is stored as a proposal. Closing
an unfinished confirmation leaves it available under Review changes.

## Choosing the useful facts

**Choose the facts to show** lists fields actually present in the cached source
metadata, with sample values. Select up to four per provider, or leave them empty
for Remy's cached selection. The preference belongs to the matching item type and
applies to items with that type's tags. It does not copy source values into owner
attributes. Configure different tagged types when different presentations help.

**Edit my fields & photos** edits your entered values. **Ask Remy to tidy my
fields** compares them with linked facts and suggests text-equivalent descriptive
values that may be redundant. Every suggestion includes the actual cached source
value. Suggestions start unchecked and require a separate removal confirmation.
Personal fields such as condition, location, serial number, format, quantity and
links are protected. Conflicting or uncertain values are kept. You can still
remove or correct them yourself in the editor.

## Runtime item types

Administration → **Item types & templates** is the one configuration used by
photo capture and manual entry. An approved template defines:

- Starting tags, registered fields, and optional default owner values.
- Which installed catalogue providers Remy may use for identification.
- Remy's type-specific identification instructions.
- Optional preferred source field paths for the concise catalogue presentation.

The common vocabulary defines owner fields. Tag definitions suggest additional
fields in either creation flow. Creating a new type makes it appear in capture
and collection navigation without a code change. New API integrations still need
a provider adapter and a licensing review; adding an arbitrary URL is not the
same as installing an authoritative data provider.

Starting fields and values are copied at creation; items have no continuing
template membership. Display preferences are selected using the item's current
tags. Identification jobs copy starting defaults, the selected provider policy and prompt so editing
a type does not silently change an already queued job. A manually chosen record
can come from any installed provider, even if that provider is not enabled for
that type's automated searches.

The manual editor's photo button enters the same durable catalogue-identification
flow as capture. It no longer generates a separate set of AI descriptive owner
attributes. The old single-photo endpoint remains for API compatibility.
