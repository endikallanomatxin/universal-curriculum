# Curriculum publishing

## Source of truth

Accepted curriculum proposals and their ordered changes are the source of truth
for published curriculum state. Each proposal points to the accepted proposal
on which it was based, forming the canonical publication lineage without a
separate version number. The `units` and `unit_dependencies` tables are a
rebuildable projection used to serve the current graph efficiently.

## Drafts and publication

The curriculum editor works on one explicit draft proposal at a time. Unit
creations, renames, deletions and dependency changes accumulate in that draft
without mutating the published projection.

Publication accepts every change in the proposal atomically. It succeeds only
when the proposal's base is still the current accepted proposal. The projection
is rebuilt by following that proposal lineage inside the same transaction.

## Reverting

Reverting does not delete or rewrite accepted history. It appends an accepted
inverse proposal and rebuilds the projection. Consequently, both the reverted
proposal and the act of reverting it remain visible in history.
