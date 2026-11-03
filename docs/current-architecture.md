# Starting point and evolution

Remventory is one Go HTTP application with embedded browser assets, a pgx-backed
PostgreSQL store, MCP tools, and Remy using Google ADK with an OpenAI-compatible
model adapter. Startup applies numbered embedded migrations. Kubernetes runs two
replicas from a snapshot image; deployment files live outside this repository.
Do not push deployment changes for development tests.

Production at the start of this work contained 100 owned records and 100 image
assets: 28 records tagged Vinyl Records and 72 tagged Movies. Six creation
templates exist (Lego Sets, Vinyl Records, Video Game, Blu-ray, DVD, VHS).
Categories became creation templates in migration 005: items no longer belong to
a template. Tags overlap and sometimes differ only in case. User-entered media
fields include artist_name, record_title and release_year alongside shared fields.
These values must survive external enrichment unchanged.

The previous Add item modal chose a template, loaded a picture, synchronously
asked a vision model for editable attributes, and then created an item proposal.
Pictures were attached after approval. The browser held the draft and processing
state; navigating away could lose the draft. Existing item and vocabulary changes
already have a durable proposal/approval boundary and should keep it.

Original photos and thumbnails use S3 (Garage in production). Some old thumbnail
reads generate a repaired derivative. Therefore pointing a development app at the
production bucket with write-capable credentials would not be read-only even
while browsing. Local development copies all referenced objects to isolated S3.

Canonical facts, identifiers, and edges now live in additional tables. They do
not modify the shared field vocabulary or owned-item attributes. Durable
identification jobs and photos precede inventory creation. External facts may be
cached before review; owned-item associations and inferred containment are only
committed through an explicit proposal decision. PostgreSQL leases and fencing
tokens allow the single application image to process jobs across replicas.
