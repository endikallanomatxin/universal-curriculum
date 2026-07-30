# Curriculum publishing

## Source of truth

Accepted curriculum proposals and their ordered changes are the source of truth
for published curriculum state. Each proposal points to the accepted proposal
on which it was based, forming the canonical publication lineage without a
separate version number. The `units` and `unit_dependencies` tables are a
rebuildable projection used to serve the current graph efficiently.

Every change has a common header that owns its proposal, position and kind. Its
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

Publication accepts every change in the proposal atomically. It succeeds only
when the proposal's base is still the current accepted proposal. The projection
is rebuilt by following that proposal lineage inside the same transaction.

## Reverting

The current reverse action is a pre-release rollback tool, not part of the
long-term proposal model. It can only remove the current accepted proposal and
then rebuilds the projection from that proposal's base. The database keeps
accepted proposals immutable outside this explicit transactional operation.

A deleted unit is not restored by a new change type. Creating similar curriculum
later creates a new unit identity; replacement and equivalence relationships
will be designed separately when those workflows are defined.
