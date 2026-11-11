# Pinterest import and repair

Imports stay on the server and continue after the user leaves Currents. They
do not need a signed-in Pinterest browser or extension.

## Two distinct listing limitations

The October 2026 production investigation found some anonymous visitor
sessions returned approximately
25 pins and ended pagination for boards containing hundreds or thousands.
Replacing only `_pinterest_sess` switched the behavior in both directions.
A working session stayed pageable across other boards; changing request
options or cache-busting a restricted session did not help.

All five large boards in a four-attempt sample produced a pageable session.
For example, one large board yielded 3,665 actual pins against an advertised
3,671, whereas its latest production import had queued only 24 images.
This is evidence for fresh-session recovery, not a permanent success-rate
guarantee. No production records were changed during the investigation.

A smaller omission persists with healthy pagination. Two fresh healthy runs
returned the same 769 of 849 pins for one board and 187 of 230 for another.
The widget endpoint still reported the advertised totals. These returned
pins all had original images; the extra item without one was a recommendation
module. Earlier investigation measured approximately 9% omitted across a
different account. The widget endpoint is diagnostic only: it caps at about
50 pins and has no useful pagination or original-resolution images.

## Retrieval and persistence

The worker retains one anonymous `PinterestClient` across an account's jobs.
A first page with at most 25 actual pins, no continuation and a larger
advertised whole-board count is suspicious. Keep the discovered images,
try up to five fresh sessions with delays, and retain a partial outcome if
all sessions remain restricted. Do not endlessly retry smaller shortfalls.

Sections and section-filtered boards cannot compare their own listings with
the whole board's total. Before using a new session, probe their unfiltered
parent board using `board_pin_count`. Historical jobs obtain that metadata
from Pinterest when repaired. A restarted listing validates its new visitor
before trusting a saved continuation cursor. Temporary network/429/5xx
failures retry the same page before ending with a recoverable partial result.

The page callback stores every image and the next cursor in one transaction.
A failed insert must leave both the queue and cursor unchanged. Never persist
the next page's cursor after only the first 100 items of a 250-item page.
An account-scoped transaction lock serializes queue deduplication; the
existing job-row lock serializes page persistence with cancellation.

`listed_count` counts actual pins in the latest listing pass, before image
deduplication. `unsupported_count` counts actual pins lacking an original
image. Recommendation modules are excluded from both. Several pins can use
the same image, so pin counts must not be used as an upload denominator.
Legacy jobs have a null listing count: their image count can identify a
suspected gap but cannot prove how many pins Pinterest originally returned.

## Repair and outcomes

`GET /api/import/sessions` returns the viewer's 20 most recent sessions, with
active sessions first. The page restores the first and lets the viewer select
a historical session. The result has one overall saved-image count, a short explanation and at most
one recovery action. Active imports communicate reading, retrying, saving,
waiting or required reconnection. Terminal outcomes distinguish completion,
missing pins, failed uploads, unsupported media and unverified older listings.
Board diagnostics stay collapsed under View board details; they have no
per-board retry buttons. Import history sits below the result and uses radio
choice cards to select a session. Selecting one closes the accordion and returns
keyboard focus to its trigger. It is hidden for a single session (the
new-import form instead offers View last import). For finished imports, Start another import is a primary
action aligned right immediately above the result card.

`POST /api/import/sessions/{id}/retry` accepts `{}` for eligible jobs in that
session or `{"jobId":"..."}` for one board. It verifies session ownership
and destinations against the PDS under the existing repository lock, skips
removed/deleting collections, and reopens terminal jobs in those exact
destinations. It resets failed item attempts and re-enumerates from page one.
Existing PDS pacing, write budgets and OAuth reconnection behavior still apply.

Queue insertion matches exact original image URLs within the same owner's
destination across jobs. Different Pinterest pin IDs can reference the same
image. Completed import history remains authoritative for whether an image
was already imported, even when its save is later moved or deleted: repair
must not recreate that membership. Other destinations are independent.
Original URLs do not identify modified/re-encoded variants of the same image;
this repair does not attempt perceptual deduplication or erase old duplicates.

Migration 060 adds listing metadata and an import-history image index. It
restarts still-listing legacy jobs from page one because their old checkpoints
could advance mid-page. It does not requeue finished historical imports.
Deploy with the normal database-backup procedure; historical repair is a
separate user action after deployment.

## Operator recovery after deployment

`appview repair-pinterest-imports --dry-run` emits a JSON plan for old jobs
that finished with at most 25 images despite a larger whole-board total. It
selects the latest session for each account/board, avoiding older duplicate
collections, section subsets with unknown totals and jobs already checked by
this release. The global plan does not contact users' PDSs.

Inspect and apply one account at a time:

```sh
appview repair-pinterest-imports --did did:plc:example --dry-run
appview repair-pinterest-imports --did did:plc:example
```

Applying without `--did` is refused so an incident recovery cannot accidentally
start hundreds of concurrent import workers. Scope broader recovery from the
global JSON plan and run only a small number of accounts concurrently.

The account check holds the repository lock, skips accounts with active
imports/deletions/wipes, requires a usable OAuth session and validates each
destination against the PDS. Missing collections are skipped. A targeted dry
run queues nothing (ordinary OAuth refresh and startup migrations still apply).
Applying only reopens jobs; the running server's watchdog starts the existing
worker, preserving upload budgets and completed-item history. The JSON report
distinguishes eligible/queued jobs, removed destinations, busy accounts and
accounts needing reconnection.
