# Review locally with a copy of the collection

The review app runs at **http://127.0.0.1:58080**. PostgreSQL and Garage run in
Docker, with ports bound only to loopback. Production credentials and the dump
are outside the repository in `~/.local/share/remventory-dev/` (directory 0700,
credential files 0600). Never commit this directory or copy its contents into an
image. Production was accessed through a read-only dump and S3 GETs only.

Start or resume the isolated services:

```sh
python3 scripts/dev-start.py
python3 scripts/dev-run.py
```

The development runner refuses remote database/storage endpoints. It loads
`~/.local/share/remventory-dev/local.json`; set `REMVENTORY_DEV_CONFIG` to use a
different private JSON configuration. The current local copy contains the 100
production inventory records and all 200 referenced original/thumbnail objects.
Two named Docker volumes preserve these independently of production:
`remventory-dev-pg` and `remventory-dev-garage`.

To take a new snapshot **into an empty local database**, first start the Docker
services, then run:

```sh
python3 scripts/dev-snapshot.py --kubeconfig ~/talos/uvy/kubeconfig
```

This reads Kubernetes secrets without modifying them, uses `pg_dump` with
`default_transaction_read_only=on`, restores only to the fixed local development
database, and runs `go run ./cmd/devcopy` to GET production objects and PUT them
into local Garage. The script refuses to replace an existing local schema. Use a
separate empty Docker volume if another snapshot is needed. Do not delete the
review database just to rerun setup. Snapshotting should be deliberate: it copies
private collection data and uses the authorized production LLM credential locally.

## What to review

1. Open **Capture & identify** or **Add item**. Choose a category and upload several
   pictures of one object. **Queue identification** saves the draft and returns
   immediately after the uploads. Leave, refresh, or photograph another object.
2. Watch queued/researching jobs become **Ready for review**. Review photos,
   inferred clues, uncertainty, source metadata, and relationships. No item has
   been created by identification. Ownership checks distinguish cached canonical
   matches from unverified original-description matches.
3. Select the correct candidate(s). A box can contain several separately resolved
   works; alternative pressings should not all be selected. Use additional source
   search to correct a match. Rejected candidates cannot be approved.
4. Review the proposal and explicitly approve it. New photos attach with the item
   in the same transaction. For existing inventory, approval adds associations
   while preserving all original personal fields and tags. You may instead keep
   the identification for search only, discard it, or retry it.
5. Open an inventory item to inspect **Associated knowledge** and source entity
   details, then search cached knowledge by work title, original year, artist or
   platform. Ask Remy “What movies do I own from 1999?” Remy has a dedicated
   canonical graph search tool. Coverage depends on approved links; a lack of
   results does not prove an unenriched work is absent.
6. **Enrich existing collection** queues a bounded batch by exact existing tag.
   Active jobs are skipped. Start with **Vinyl Records**. Results use the same
   worker, source cache and approval path as new photos.

The initial source review, compatibility matrix and limitations are in
[external-sources.md](external-sources.md). Front covers commonly establish an
album but cannot distinguish a pressing. Back covers, catalog numbers and record
labels help; Remy should say when that evidence is absent.

## Verification

```sh
go test ./...
TEST_DATABASE_URL='postgres://postgres:remventory-local@127.0.0.1:55432/remventory_dev?sslmode=disable' go test ./...
REMVENTORY_LIVE_PROVIDERS=1 go test ./internal/authority -run TestLiveAuthorities -v -count=1
```

Database integration tests create and remove isolated schemas. The canonical
integration suite refuses non-loopback databases. Live provider tests are opt-in
and make rate-limited real calls. Pause local workers before running a separate
live-test process so the two processes do not independently consume the same
MusicBrainz IP rate limit. Ordinary unit tests do not use external APIs.

`REMVENTORY_WORKER_ENABLED=true` enables the in-process background worker. There
is no second image or broker. Each application process handles one job at a time;
Postgres leases coordinate processes. Interrupted jobs become claimable after a
12-minute lease; normal failures retry up to three attempts with a delay, then
require manual retry. Application shutdown cancels active work and saves retry
state before closing the pool.
