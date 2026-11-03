# External authorities

Reviewed against the official sources linked below on 2026-11-03. An accessible
API is not an open database. No external artwork is downloaded: uploaded photos
remain the owner's photos. Unknown terms mean no integration, not permission.

| Provider | Domain | Open data? | API/dataset | Auth | Rate limit | Cache allowed? | Commercial use? | Identifier | Relationship depth | Recommendation |
|---|---|---|---|---|---|---|---|---|---|---|
| MusicBrainz | Music releases, groups, recordings, works, artists, labels | Core CC0; supplementary BY-NC-SA 3.0 | REST JSON/XML, database dumps | None for public reads | 1 request/sec/IP; descriptive User-Agent | Yes, core indefinitely | Core yes; public service commercial use requires separate terms | UUID MBID scoped by entity type, persistent/redirectable after merges | Pressing → release group, media/tracks → recordings/works; artist/label relationships | Implement core only; strongest initial music authority |
| Wikidata | Films, TV, episodes, games, hardware, people, organizations | Structured data CC0 | Action/REST APIs, entity JSON, dumps, SPARQL | None for public reads | Dynamic Wikimedia limits; serial requests, maxlag and backoff; query service separate limits | Yes, indefinite | Yes | QID, persistent, merges/redirects possible | Cross-domain graph; inconsistent physical-release coverage | Implement; never equate a film QID with a physical Blu-ray release |
| Discogs | Excellent music pressing coverage | Catalog fields CC0, other content restricted | API, monthly dumps | Search requires token/OAuth | API documentation: typically 60 authenticated/25 unauthenticated per minute; verify before enabling | API terms impose six-hour display freshness and service-necessity caching language | CC0 yes; restricted data no | Numeric release/master/artist/label IDs | Release/master, track/artist/label; generally weaker work graph | Defer API: CC0 catalog is promising, API freshness terms conflict with offline-first cache; investigate dump-specific rights separately |
| TMDB | Films, series, seasons, episodes | Proprietary, free API is not open data | REST | API key/bearer | Soft upper limits around 40/sec; respect 429 | No unconditional perpetual redistribution right established | Separate commercial license | Numeric IDs scoped by type | Strong TV hierarchy, weak physical products | Defer; attribution/logo and commercial terms; Wikidata first |
| IMDb | Titles, people, episodes | Not open | Daily subset TSVs, commercial offerings | None for subset downloads | Download service, no public general API in subset | Personal noncommercial local copies allowed | Not under noncommercial dataset terms | tt/nm IDs | Episode/series and credits, no pressing catalog | Reject as foundation; retain source-asserted alternate IDs only |
| IGDB | Games, versions, platforms, releases | Proprietary | Twitch-backed API | Twitch client ID and OAuth | 4/sec, 8 open requests | No open perpetual dataset grant established | API docs now say free for commercial and noncommercial projects, attribution requested | Numeric IDs scoped by endpoint | Rich edition/platform/release graph | Defer; useful supplement, not open authority |
| MobyGames | Games, editions, releases, platforms | Proprietary | Subscription API | Paid key | Noncommercial 720/hour, max 1/sec | Contract-specific, not established as unrestricted | Commercial tier | Numeric game/platform IDs | Detailed releases/credits | Defer: paid proprietary foundation unnecessary initially |
| EIDR / ISAN | Audiovisual works/versions | No open reusable full dataset established in this review | Registry services | Membership/registration/service-specific | Not verified | Not verified for intended cache | Contract-specific | DOI-like EIDR / ISAN | Work/version identifiers, incomplete retail releases | Do not integrate until current access/license/cache terms are verified |
| Open Library | Books, editions, authors | Catalog intended as open/public-domain data | APIs and dumps | Public reads no key | Official default 1/sec, identified clients 3/sec | Catalog data yes; cover rights separate | Catalog yes | OL…W / OL…M / OL…A | Work → edition → author | Future provider; no need to add book-specific behavior before real examples |

MusicBrainz API results are not a blanket CC0 payload. Remventory requests no
ratings, tags, annotations, user data, or cover art and stores selected core
fields and core relationships only. Source credit and canonical links are shown
even where attribution is not legally required. Public API use here is personal,
low-volume inventory use; commercial deployments must review MetaBrainz service
terms independently of CC0 data rights. Source identifiers in Wikidata claims
are source assertions; they do not constitute validation by the named service.

## Official references

- [MusicBrainz data licenses](https://musicbrainz.org/doc/About/Data_License),
  [core versus supplementary data](https://musicbrainz.org/doc/MusicBrainz_Database),
  [API](https://musicbrainz.org/doc/MusicBrainz_API),
  [rate limits and User-Agent](https://musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting).
- [Wikidata licensing](https://www.wikidata.org/wiki/Wikidata:Licensing),
  [data access](https://www.wikidata.org/wiki/Wikidata:Data_access),
  [MediaWiki API etiquette](https://www.mediawiki.org/wiki/API:Etiquette).
- [Discogs API terms](https://support.discogs.com/hc/en-us/articles/360009334593-API-Terms-of-Use),
  [developer documentation](https://www.discogs.com/developers).
- [TMDB FAQ](https://developer.themoviedb.org/docs/faq),
  [rate limiting](https://developer.themoviedb.org/docs/rate-limiting).
- [IMDb noncommercial datasets](https://data.imdb.com/non-commercial-datasets/).
- [IGDB API documentation](https://api-docs.igdb.com/).
- [MobyGames API](https://www.mobygames.com/info/api/).
- [Open Library APIs](https://openlibrary.org/developers/api),
  [licensing](https://openlibrary.org/developers/licensing).

Coverage is uneven: MusicBrainz can have many almost identical pressings, missing
barcodes and incomplete recordings/works. Wikidata often identifies a film or
game reliably but cannot verify a particular retail box. Missing retail entities
remain missing; Remventory can propose several `contains` links from an owned
box to independently verified works, with the packaging interpretation explicitly
recorded as an inference approved by the user. It must not invent a release QID.
