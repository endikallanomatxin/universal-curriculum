# Curriculum publishing

## Source of truth

Accepted curriculum proposals and their declarative changes are the source of truth
for published curriculum state. Each proposal points to the accepted proposal
on which it was based, forming the canonical publication lineage without a
separate version number. The `units` and `unit_dependencies` tables are a
rebuildable projection used to serve the current graph efficiently.

Proposal authorship is a many-to-many relation. New drafts currently
start with their creator as their sole author, while the storage model allows
later composition workflows to retain every contributing author.

Every change has a common header that owns its proposal and kind. Its
payload lives in a type-specific table for unit creation, rename, content
update, deletion or dependency mutation. Database constraints require exactly
one payload of the declared kind.

The globally unique ID of a unit's creation change is also the unit's stable ID.
This makes the creation event the durable identity and provenance of the unit,
without a second identifier. Later changes, learning paths and progress all
reference that creation even while it is hypothetical or after it has been
retired. Storage does not restrict such references to the same proposal,
publication lineage or acceptance status.

## Drafts and publication

The curriculum editor works on one explicit draft proposal at a time. Unit
creations, renames, deletions and dependency changes accumulate in that draft
without mutating the published projection. A hypothetical unit receives its
stable ID as soon as its creation change is stored, so subsequent changes in the
draft can refer to it normally.

Submission freezes every change in the proposal for administrator review.
Acceptance applies every change atomically. It succeeds only when the
proposal's base is still the current accepted proposal. The projection
is rebuilt by following that proposal lineage inside the same transaction.
Recognition changes also materialize the progress they grant from the evidence
that predates the proposal before the transaction commits.
PostgreSQL also enforces that proposal bases are accepted, that publication
extends the locked projection head, and that the projection advances to that
direct successor before commit. These constraints keep the canonical history
linear even if persistence is called outside the normal service workflow.
Before acceptance, changes are applied in a canonical order independent of how
the author entered them: unit creations; name and content edits; dependency
removals; dependency additions; recognitions; and unit deletions. Unit and
prerequisite IDs, followed by the change ID, provide deterministic tie-breaks
within a phase. Every operation must affect the state it declares, all
referenced units must be valid for its phase, and the resulting graph must
satisfy the product invariants. Previous values and deleted-unit snapshots are
derived by applying the same canonical order over the proposal's frozen base
rather than duplicated in change storage.

Draft editing keeps the proposal as a normalized diff from its base. Reversing a
dependency change removes that change instead of recording the opposite change.
Discarding a unit created by the draft removes its creation and every change in
that draft which references it. Deleting a published unit similarly drops draft
name, content and outgoing-dependency changes that the deletion supersedes.

## Rebasing drafts

After a proposal is accepted, every other draft is considered independently.
The system follows the accepted lineage from the draft's frozen base to the new
head and calculates the unit footprint of both lines of work. Unit changes touch
their unit, dependency changes touch both endpoints, and recognitions touch all
sources and targets.

If the footprints are disjoint and replaying the complete draft over the new
head passes publication validation, its base is advanced automatically in a
separate transaction. A failure to inspect or rebase one draft never rolls back
the proposal that was already accepted. The same automatic check runs before a
later edit or publication as recovery from an interrupted maintenance pass.

An overlapping draft remains unchanged on its historical base. Its workspace
shows the accepted proposals responsible for each overlap and asks the author
to resolve every conflicting draft change. Content conflicts use one editable
merge document initialized to the proposal's intended result. Unchanged source
appears once; each differing passage shows the editable resolved result beside
the accepted alternative and an immutable copy of the proposal's intended
passage. Either reference can replace that local passage directly, so editing
never removes the author's original proposed version. A single unrestricted
source field is the canonical client-side result. The browser regenerates the
comparison from its latest value whenever that view opens; the backend supplies
the two references and receives only the resolved content. Other changes can
be kept or dropped. Dependency changes that already have their desired result
become no-ops. The normalized proposal is validated as a whole
before its base moves. Until then ordinary curriculum edits and publication are
blocked, while proposal metadata and draft deletion remain available.

The proposal workspace combines metadata, changes, submission and rebase
resolution. Accepted proposals form a single publication line in the history
view, with current drafts rendered as branches from their recorded bases.
Proposal title and rationale are document-like inline fields which autosave
after a short input delay; their list and breadcrumb mirrors update immediately
without replacing the workspace or moving focus. Status, author and change
count precede the rationale as a compact vertical definition list. Submission
and deletion are terminal draft actions and therefore remain at the end of the
full proposal workspace rather than alongside frequently edited metadata.

## Reviewed history

Submitted, accepted and rejected proposals and their changes are permanently
immutable. Rejection records an administrator's reason without changing the
curriculum. Rejected proposals remain readable by their authors and
administrators as historical reference. Mistakes in the published curriculum
are corrected through a subsequent proposal based on the current accepted
state, preserving the complete publication history.

A deleted unit is not restored by a new change type. Creating similar curriculum
later creates a new unit identity; replacement and equivalence relationships
will be designed separately when those workflows are defined.
