# Using Remventory

Tell Remy what you want to accomplish in the visible composer: “Find my Wii games,” “Compare these items,” or “Propose adding a description to this item.” Remy can present cards, details, comparisons, statistics, and changes for review. Inventory changes and new vocabulary definitions require your approval.

The visual collection also supports direct browsing. Select several tags to find items carrying all of them. Text search finds values in fields and tags.

## Define fields and suggestions

Open **Administration → Common fields** to see the shared vocabulary and choose **New common field**. Fields such as Title, Description, Year, and Links are predefined. Define another field with a stable key, label, and value type, then review and approve it before using it on items.

Under **Administration → Tag defaults**, choose **Manage tag defaults**, enter a tag, and select existing field definitions. Review and approve the association. Items with that tag can use those fields or any other approved definitions; none of their values is mandatory.

Links are a list of named web references. Add an IMDb page, Wikipedia article, manufacturer page, and other references as entries in Links rather than creating a field name for every site.

## Create templates

Under **Administration → Templates**, choose **New template** to select starting fields, fill optional default values, and choose default tags. Review and approve the template.

Choose **Edit** beside a template to change its starting fields, defaults, name, or tags. **Delete** opens a deletion review; existing items keep their details.

Use the template selector in **Add item** to start a new item. Its fields, values, and tags are copied into the draft. The item is independent afterward: you can remove defaults, add other fields, or change tags without affecting the template or other items.

## Add an item

Choose **Add item**, then select a template or **Continue without a template** before adding pictures. Remy uses the selected template’s fields, defaults, and tags as context when identifying your photo. Choose several JPEG, PNG, or GIF pictures up to 12 MB each, or take a picture. Export HEIC as JPEG first. Use **Use as primary** to select the image shown on the collection card. Phone-photo orientation is respected, and previews crop from the center without changing the original photo.

Remy automatically suggests details when you add a new picture to a new item. A progress panel beside the photo shows when Remy is reading it and which template is being used. Turn off **Let Remy suggest details when I add a photo** to enter details manually, or use **Suggest details from picture** to request suggestions yourself. Suggestions fill empty fields and preserve your entered values. A local model may take a few minutes to load. **Stop reading** cancels identification and keeps your draft. Check inferred information before approving it. New field names suggested by a model need their own vocabulary proposal before they can be used.

Enter tags and pick approved fields. Title is an ordinary optional field, and you can remove it. Location and Quantity are available through the same picker if useful. You can also save an item without metadata and identify it by its pictures and tags.

Review the item, pictures, and primary selection, then approve. Keep the draft open while pictures upload. If saving a picture fails after the item is saved, retry from the same dialog. Closing the dialog or reloading discards its unsaved draft.

## Maintain the collection

Open a card to view its details, named reference links, and pictures. **Edit details** can change tags, add or remove registered fields, and add pictures. Select another primary picture and approve the change. **Delete item** opens a separate review; deletion happens only after **Approve deletion**. The internal database ID is not shown as item metadata.

**Review changes** shows pending proposals from Remy or earlier interrupted work. Approve or reject each explicitly. Remy has no approval tool.

Reviews show readable fields and values. Payloads and event information are collapsed under **Technical details** or **Proposal details**. Structured model output and trace markup that reach the answer view are also tucked into expandable response details.

Browsing and manual cataloging need Postgres but no model. Saving photos requires S3-compatible storage; identification separately requires a vision-capable model.
