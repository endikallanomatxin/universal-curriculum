# Curriculum versioning

## Source of truth

Accepted curriculum proposals and their ordered changes are the source of truth
for published curriculum state. The `units` and `unit_dependencies` tables are
a rebuildable projection used to serve the current graph efficiently.

## Drafts and publication

The curriculum editor works on one explicit draft proposal at a time. Unit
creations, renames, deletions and dependency changes accumulate in that draft
without mutating the published projection.

For the current MVP, publication accepts every change in the proposal
atomically. The projection is rebuilt from accepted proposals in publication
order inside the same transaction.

## Reverting

Reverting does not delete or rewrite accepted history. It appends an accepted
inverse proposal and rebuilds the projection. Consequently, both the reverted
proposal and the act of reverting it remain visible in history.
