# Production rollout and rollback

The user authorized production deployment and a small enrichment sample on
November 3, 2026. The application image is built by the main-branch workflow;
worker configuration and writable temporary storage were applied to the existing
deployment. Preserve original fields and use proposals for inventory changes.

## Application and database

The existing Go application/image now also runs an optional durable identification
worker. New HTTP/MCP capabilities support open authority search/resolution,
canonical graph search, durable capture, existing collection analysis, and
reviewed associations. Remy can search cached original work dates and related
entities. The browser adds rapid capture, processing/review queues, source detail,
manual candidate search, and per-item enrichment. Existing manual editing and
proposal flows remain available.

Migration **006_canonical_intake.sql** adds canonical entities, scoped external
identifiers, separately cached metadata, source relationships, owned-item links,
identification jobs/photos, and shared provider rate gates. Migration
**007_canonical_search.sql** adds cached search indexes and refresh indexing.
Both are additive. Neither rewrites existing inventory attributes, tags, category
assignments, original photos, or pending proposals. Owned associations require an
explicit proposal decision. All selected associations, new item creation and
photo attachment commit atomically. Existing-item enrichment only adds links.

Before rollout take a fresh database backup and verify restore separately.
Startup migrations currently run in the application. The migration runner now holds a session advisory lock for the entire
migration sequence so concurrent replicas serialize startup migrations. Still
verify them first on a restored snapshot and use a controlled rollout.

## Configuration and Kubernetes

- Keep the existing PostgreSQL and S3 settings and existing vision/model secret.
- New setting: `REMVENTORY_WORKER_ENABLED=true`. Default is false to avoid starting
  work merely by installing an application build. Enable deliberately after the
  migration and UI review. One worker per enabled process is safe across the two
  existing production replicas: claims use `FOR UPDATE SKIP LOCKED`, expiring
  leases and fencing tokens. No broker, separate image, or volume is needed.
- MusicBrainz and Wikidata public read APIs need no new secrets. External HTTPS
  egress must permit `musicbrainz.org`, `www.wikidata.org`, and
  `query.wikidata.org`, plus the existing model endpoint. Check cluster-wide
  egress policies; keep PostgreSQL/Garage connectivity as presently configured.
- Confirm ingress body-size/timeouts allow up to eight 12 MB photographs plus
  multipart framing. Upload requests finish after durable storage, while the LLM
  runs outside the request. Avoid buffering these uploads into a read-only root
  filesystem without a writable temp directory.
- The existing deployment uses `readOnlyRootFilesystem: true`. Multipart parsing
  can spill to disk, so add a size-limited `emptyDir` mounted at `/tmp`, or set an
  equivalent writable `TMPDIR`. The repository has a proposed patch in
  `dev/production-worker-patch.yaml`. It has been applied to production and
  recorded in the local Talos repository, preserving prior unrelated edits.
- Add resource requests/limits suitable for concurrent image decoding; originals
  allow 50 million pixels and uploads are bounded. Consider ingress concurrency
  limits if capture volume rises. There is no arbitrary file upload support.

## Storage, cache and failures

Draft originals/thumbnails use `intake/<uuid>/...` in the existing S3 bucket.
Approval reuses these objects; it does not rewrite originals. Existing enrichment
reads the existing photos. Rejection keeps the audit and photo draft; no automatic
production deletion or retention cleanup is included. Define retention policy
before high-volume use. Development uses a fully isolated local Garage copy.

Cache metadata is retained locally and searched without live authority calls.
Initially source metadata becomes eligible for refresh after 90 days. New
identification/resolution consults the cache; there is no request-time graph-wide
refresh or scheduled crawl. Outages preserve the previous cache. API requests are
serialized through PostgreSQL provider gates at 1.2-second intervals with
throttling backoff. This gate coordinates these application replicas, not other
programs sharing the same public IP. Live tests should not compete with workers.

Jobs have a 12-minute lease, a nine-minute processing budget, at most three
automatic attempts, and a retry delay. Manual retries are available. Source
traversal is deliberately bounded; incomplete metadata must not imply absence.
Wikidata provides work-level identity but often no home-video release identity.
The user's approved association records packaging interpretation rather than
inventing a retail identifier. No proprietary API is enabled.

## Rollback and enrichment

Disable workers before rollback. Revert the application image while leaving the
additive tables intact; old code continues to read original inventory. Do not
drop canonical/job/proposal history to roll back. Changes to owned data that a
user approved are normal durable inventory; restoring a backup would discard
subsequent legitimate writes and needs a separate decision. Existing enrichment
preserves personal fields. Keep new proposal types in mind when using an old UI:
it cannot adequately present identification proposals.

After local approval, deploy the application and migrations deliberately, smoke
test one new capture and one existing item, then start with a small Vinyl Records
batch. Review source/pressing ambiguity and approve selected associations before
expanding. Do not auto-approve every high model confidence: front covers rarely
prove a pressing. Use the same job/proposal infrastructure for movies and sets,
then review tag normalization separately. Case variants and genre tags should
not be rewritten merely because an external source has different classifications.

Unresolved decisions: photo/job retention; whether commercial installations
should use a MetaBrainz service agreement or local dump; later open release-level
sources for home video/hardware; broader TV child traversal beyond bounded pages;
and user-reviewed equivalence/merges across providers. The prototype access token
remains the existing access-control model.


## Rollout observations

The first image build and automated unit/security checks succeeded. The rollout
trigger's GitHub OIDC token acquisition failed with a socket hang-up, so the
approved deployment used `kubectl rollout restart` after the image was published.
This trigger failure is independent of application startup and migrations.

A final live provider run exposed Wikidata's anonymous 50-ID request limit on a
large TV hierarchy. Entity retrieval now splits larger bounded traversals into
batches of at most 50; a regression test verifies all 80 entities survive batching.


## Guided intake update — November 4, 2026

The user authorized production iteration, including data curation. Migration 008
adds only canonical_presentations for cached model-selected field plans. AmiiboAPI
adds outbound HTTPS to amiiboapi.org and bounded private reference-image reads
from raw.githubusercontent.com; existing internet egress covers both. No new
secrets, worker processes, buckets or Kubernetes configuration are needed. Owner
photos remain the UI artwork. Presentation choices never execute model HTML and
never replace source facts or owned-item attributes. Existing tag normalization
is performed through proposals with exact before/after field checks.

## Unified catalogue and type management (migration 009)

Migration 009 adds `categories.intake_config`, upgrades the existing template
configuration, seeds missing starter types, and suggests existing common personal
fields in those templates. It preserves every owned item's title, attributes,
photos and canonical associations. No production back-catalog field cleanup runs
automatically; Remy's cleanup suggestions require owner selection and approval.

The app and worker use the same image. No new Kubernetes resources, credentials,
object-storage layout, or provider dependency is needed. Manual catalogue changes
use the new `catalogue_change` proposal kind; generic references and field removals
use existing item proposals. Both remain explicit-confirmation operations.

Take the normal database backup before rollout. The migration is additive and can
remain in place if the application image is rolled back. Finish or reject pending
`catalogue_change` proposals before rolling back to an older binary that does not
understand that proposal kind. Approved links remain compatible with the older
schema. Type-policy settings are ignored by older app images. Existing inventory
and source caches need no destructive migration.
