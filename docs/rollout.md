# Production rollout review — not executed

Production remains unchanged. Do not commit or push the deployment repository
until the local experience, results and migration plan have been approved.

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
  `dev/production-worker-patch.yaml`; review it and the current deployment before
  applying. It has not been copied into the deployment repository or applied.
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
