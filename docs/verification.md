# Canonical intake verification — November 3, 2026

The local database was restored from a read-only production snapshot: 100 owned
items, including 28 Vinyl Records and 72 Movies records, and 100 image assets.
All 200 referenced original/thumbnail objects were copied into isolated local
Garage. Production images were not overwritten or deleted.

## Checks

- Ordinary Go tests and `go vet ./...` passed.
- PostgreSQL integration tests passed against isolated schemas in Docker
  PostgreSQL 18, including additive migrations, approval transactions, original
  field preservation, original-year traversal, duplicate approval rejection and
  stale-worker fencing.
- Live MusicBrainz/Wikidata integration tests passed for Thriller and Kind of Blue
  pressings, The Matrix, a film trilogy, Friends, Breath of the Wild, a controller,
  and a nonexistent object. Wikidata requests respect the 50-ID anonymous limit.
- Browser checks passed for capture, queue/review navigation and review rendering.
  A production upload of two photographs survived refresh, finished research,
  and was retained as search-only. It created no owned item.
- Production migrations 006 and 007 were verified in schema_migrations. Both
  replicas became healthy on the new application with the worker enabled.
- The pre-rollout backup was restored separately and verified to contain all 100
  items and 100 image records.

## Existing collection sample

Ten production-derived vinyl records were researched locally using their real
photos and original fields. One had a confident catalog-number match (Johnny
Mathis), five had plausible/ambiguous results, and four had no supported match.
These are small-sample findings, not an estimated whole-catalog match rate.
Front-cover-only photos commonly identify an album without establishing a
pressing. Disney and recent soundtrack coverage was uneven. Matrix/TV examples
were also researched locally; traversal is bounded and does not promise complete
episode coverage.

A deliberately small production cleanup then used the explicit proposal and
confirmation endpoints:

| Owned record | Approved change | Evidence / limits |
|---|---|---|
| Give Me Your Love for Christmas — Johnny Mathis | MusicBrainz release association | Visible Columbia Stereo CS 9923; artist, album, track list and 12-inch vinyl agree. Record-label/matrix-runout details remain unverified. |
| The Matrix Collection | Four Wikidata film associations: The Matrix, Reloaded, Revolutions, The Animatrix | All four names are visible on the box spines. Bare-title authority search corrected the model's overly qualified empty searches. Retail edition and Matrix Experience content remain unresolved. |
| The Matrix Collection | Add Box set tag | Visible multi-disc box; existing Movies and DVD tags retained. |

Production cached searches found the Matrix box for both 1999 and 2003 film
queries, and found the record for a Johnny Mathis artist query. The collection
remained at 100 items. All original titles and attribute values were compared
with the pre-rollout API baseline and preserved; the only intended tag change
was adding Box set to the Matrix collection.

The Matrix search exposed an orchestration detail: Wikidata entity search is a
label/alias lookup, not web search. The identification instructions now explicitly
require bare titles/model names and disambiguation using returned descriptions.
A failed qualified search must not be interpreted as proof a work is absent.

The first rollout trigger failed to acquire a GitHub OIDC token. A direct rollout
was requested, and later automatic triggers succeeded. Kubernetes controller
reconciliation was temporarily delayed, while existing replicas continued serving.
No cluster-wide configuration or control-plane restart was performed.

The small pre-rollout backup is retained privately under
`~/.local/share/remventory-backups/`; it is not committed or included in an image.
Development containers, their test volumes and `~/.local/share/remventory-dev/`
are disposable after production verification. Future development can recreate an
isolated snapshot using the documented scripts.

The tag-only production edit exposed an existing HTTP proposal path that replaced
attributes instead of preserving omitted values. It temporarily cleared the
Matrix item's attributes; a corrective proposal immediately restored the exact
pre-rollout title and attributes, retaining the intended tag and canonical links.
The shared store now merges partial updates, the browser submits explicit field
removals, and approvals reject stale field snapshots. Regression tests cover
preservation, deliberate removal, and stale proposals. The final baseline check
must compare all 100 items again after the corrective change.

## Guided Amiibo intake — November 4, 2026

The new intake uses Type → Photos → Match → Save, with one decision per screen.
A live AmiiboAPI test verified distinct Mario series and exact head/tail lookup.
Private visual reference comparison corrected a model-only series misclassification:
character recognition alone is insufficient, so Amiibo confidence now requires
matching the sculpt/pose to a verified catalogue reference or exact identifying
text. Reference artwork is held in memory for recognition and not shipped in the
UI; the owner photo is the review image.

Against an isolated production-derived database, browser review and approval
linked a test Mario to `0000000000000002` (Super Smash Bros.). The resulting owned
item was searchable by cached series metadata. This test figure is not a claim
that the user owns one and is not copied into production inventory. The review
card showed only its name, series, game series, type, confidence and source.
Desktop/mobile checks cover the visible steps and adjacent primary action.
Presentation tests reject invented fields, preserve cached decisions for unchanged
source data and invalidate them when source facts change. PostgreSQL migration
008 stores these decisions separately from provider metadata and personal data.
