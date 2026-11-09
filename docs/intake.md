# Add an item with Remy

Choose **Add item**, then follow **Type → Photos → Match → Save**. Each screen
shows the current decision and its next action. For Nintendo Amiibo, choose
**Amiibo**, photograph the figure and optionally its base or packaging, then
choose **Find this item**. Photos and identification are durable after submission;
**Pending items** lets you return later or photograph another object immediately.

For video games, include the console banner and edition text in your photos.
When the selected template includes the **Console** field, the Save screen prefills
an editable console suggestion from the photo analysis and shows the game's
catalogue platforms. Keep the suggestion or change it, then use **Add to my
inventory** to accept the title, console and tags together; there is no separate
console checkbox. If the photo is inconclusive and several platforms are listed,
choose the console shown on your copy. Nintendo Switch and Nintendo Switch 2
are separate choices.
If a console is missing from your shared choices, type its name and use
**Approve new console choice** before confirming your copy. This approves a
vocabulary proposal; **Add to my inventory** separately confirms the item.
The confirmed console is also added as a tag; duplicate tags and the generic
Consoles default are removed for this intake. Existing pending scans reuse their
saved photo analysis, so they usually need no reprocessing or new photos. Visible
edition wording is retained in the suggested name when it matches the selected game.

Remy looks up specific figure variants in AmiiboAPI. A Mario character match
alone is insufficient: the Super Smash Bros. figure and Super Mario Bros. figure
have different identifiers. Where available, private catalogue reference images
help Remy compare pose, sculpt, colour and base. Uncertain variants need review.
A reference image is recognition input, not an application illustration or a
claim of ownership. The UI displays your uploaded photo.

The match card shows the verified catalogue identity, a few relevant facts and
the source link. Remy selects the field order and presentation from real source
fields; that decision is cached separately from source metadata. Source changes
invalidate the cached selection. No model-supplied HTML or invented fact values
are accepted. Repeated viewing does not ask the model to redesign the card.

Check the match, continue to Save, and use **Add to my inventory** to confirm.
That explicit action creates and approves a proposal, attaching the photos and
canonical record in one transaction. For existing items, **Find a match** uses
the same flow; **Save match** attaches external knowledge while preserving your
personal fields. Existing active research is resumed rather than duplicated.

For an unsaved pending item, choose **Add photos and try again** on the match or
failed-identification screen. Take or choose another view of the same object,
then choose **Search again with these photos**. Remy keeps the original photos
and searches again using all of them (up to eight total). Cancel leaves the
pending item unchanged. Wait for an active search to finish before adding photos;
the new result still needs your review and confirmation before saving.

An uncertain result can also be searched by character/title, retried, or discarded.
The system distinguishes a photographed product variant from its character or
underlying film/work. Box-set review lists contained titles rather than all
cached graph nodes. Detailed source data remains cached and searchable but is
not dumped into the review interface or copied into your personal attributes.

The collection sidebar shows broad collections. Genre, format and other legacy
tags belong in organization details, rather than dominating navigation. Data
cleanup uses proposals and preserves personal fields and owned-item identities.

Remy's choice is a suggestion, not a gate. Use the version arrows or name search
to select another source-verified option, including one the model thought was
less likely. The Save screen shows your chosen series. Confirming that choice
records human confirmation separately from the model's earlier assessment.
Reference-image comparison is optional and time bounded; uncertain candidates
can still proceed to human review.

The processing screen keeps the photo in place while an htmx HTML fragment polls
server-side progress. Leaving the screen does not stop the durable job. Pending
items retain their layout between unchanged updates. htmx is bundled locally.

Suggested alternatives show their names and series. Check the important source
facts, choose a version, then confirm the final selection. Optional personal
fields can be added before saving. Later, **Edit my details** changes your own
fields and photos; **Correct catalogue match** starts another review. The final
review offers an explicit checkbox to replace previously linked matches. This
changes ownership associations, not shared source metadata or your personal data.

Collection cards use both color and text: green **Identified** means an approved
catalogue link exists; amber **Needs review** means results await confirmation;
blue **Identifying** means server work is pending; gray **Not identified** means
there is no approved catalogue link. An identified work does not imply that its
physical edition or pressing has been confirmed.

All uploaded views remain available from the photo selector during review. The
item detail shows a short **Your copy** section for stored format, condition and
location, with other personal fields under the expandable notes section.
