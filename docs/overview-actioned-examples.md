# Overview actioned examples

Three GitHub-cluster cases that lock how **Mark as deletable**, the live month board, the deletion cart, and a bottom **actioned** strip should behave. Classify and Mark stay report-only. Mailbox mutation stays on confirmed **Move to Trash at source**.

The overview page follows these cases: Mark skips in-cart and actioned hashes, live months drop Trash or trashed hashes at serve start, and the bottom Actioned strip lists sender, count, date range, and outcome. Checkout still drops cart rows whose replica folder is Trash, then empties the rest of the cart.

## Surfaces

| Surface | What it is on this machine |
|---------|----------------------------|
| Live months | Nov 2026 board: **What it costs you**, sender minutes, kind tree. HTML is baked at `mail overview serve` start. |
| TLDR and delete safety | Highlight a region, then **Mark as deletable**. Today this POSTs `/cart` with the baked hashes. |
| Deletion cart | Staging list plus **Move to Trash at source** checkout. The job line is the order tracker after the cart empties. Live JSON. |
| Actioned strip | Not built yet. A shorter board under the live months: sender, count, date range, outcome. Not a second kind tree. |

Outcome values on the actioned strip: `trashed at source` (job item `trashed` after Move) or `already in Trash` (replica collection is Trash). Failed Move is not an actioned outcome.

## Example 1: still in All Mail

**Cluster:** GitHub notifications for Nov 2026. Sender `notifications@github.com`. 83 messages, 1 Nov through 7 Nov. Replica collection `gmail/imap/[Gmail]/All Mail`. Not in the cart yet.

| Surface | After Mark |
|---------|------------|
| Mark as deletable | Enabled. POST `/cart` stages the 83 hashes as one group. |
| Live Nov board | Still counts 83 messages and 6.92 minutes for that sender. Kind tree unchanged. |
| Deletion cart | Shows the group (Cart 83). Move checkouts them. |
| Actioned strip | Empty for this cluster. |

This is the first Mark of unread GitHub. The live board keeps costing time until Move succeeds and the replica shows Trash.

## Example 2: Move failed

**Cluster:** Same Nov GitHub 83. Screenshot job `55b84bfbd12386f4`: status `partial`, planned 0, refused 0, trashed 0, failed 83. Bindings used folder `gmail/imap/[Gmail]/All Mail`. Himalaya returned `No message deleted: no id matched in the mailbox` for each envelope id.

They are not deleted. Pimdir still presents All Mail. The envelope ids did not match Himalaya's mailbox.

| Surface | After the failed Move |
|---------|------------------------|
| Mark as deletable | May restage; hashes are no longer in the cart after checkout. |
| Live Nov board | Still counts 83 messages and 6.92 minutes. Failed is not "done". |
| Deletion cart | Empty after checkout. Job stays `partial` / failed 83 on the job line. **Move to Trash at source** retries that leftover job after ids are repaired. |
| Actioned strip | Must not list this cluster. Failed is not actioned. |

## Example 3: already in Trash

**Cluster:** GitHub notifications for the past week (Sep 2026), 115 of the 183-hash cart that later resolved to `gmail/imap/[Gmail]/Trash`. Job `b986da0ab3a4bcc5`: planned 0, refused 183, trashed 0, failed 0. Checkout drops those cart rows and leaves the job as the tracker.

Replica last-wins Trash. Himalaya must not run `message delete` on Trash.

| Surface | After checkout sees Trash |
|---------|------------------------|
| Mark as deletable | Must not stage those hashes again. POST `/cart` skips or refuses Trash-folder hashes. |
| Live months | Must stop counting those hashes. Sep sender minutes, kind tree, and costs drop the 115 (and the rest of that 183). |
| Deletion cart | Empty of those hashes (checkout removes refused Trash rows, then clears the cart). |
| Actioned strip | One concise row: sender `notifications@github.com`, count 115, date range 19 Sep through 30 Sep, outcome `already in Trash`. Sibling senders from the same 183 (GitLab, LinkedIn, Seek) get their own short rows, not a kind tree. |

## Rule table

| Replica / job | Mark | Live months | Cart | Actioned strip |
|---------------|------|-------------|------|----------------|
| All Mail, not in cart | Stage the hashes | Count them | New group | Omit |
| All Mail, staged in cart | Do not restage | Count them | Keep until checkout | Omit |
| All Mail, job failed after checkout | May restage | Count them | Empty; retry Move on leftover job | Omit |
| Trash, or job item `trashed` after sync | Do not stage | Omit the hashes and recount | Drop | Concise row (sender, count, range, outcome) |

## Keep (worth reading)

Overview **Keep (worth reading)** on the same highlight panel writes `kept` to `message_outcomes`. It does not hide rows on live months (only trash outcomes do). Trash labels still win if a hash is later moved to Trash.

## Out of scope

- Auto Move or auto-trash from Mark, Jev, or classify.
- A second kind tree on the actioned strip.
- Treating failed Himalaya ids as deleted.
- Hiding still-live All Mail rows because they were marked.
