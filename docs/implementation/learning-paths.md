# Learning paths

Authenticated users can maintain several personal learning paths. A path stores
a name and a set of target unit IDs.

The default graph for a path is the induced subgraph containing its targets and
every published prerequisite that can lead to them. Focusing a unit applies the
same local-neighbourhood navigation used by the full curriculum, but never
escapes the path subgraph.

Learning paths reference the stable ID of each unit's creation change rather
than the current `units` projection. Retiring a unit therefore preserves it as a
path target and marks it unavailable unless an accepted recognition migrates
that target. A later creation receives a distinct identity rather than silently
inheriting a retired target without an explicit recognition.

When a proposal is accepted, each path target matching any recognition source
adds every target of that recognition. An active source remains selected; a
source is removed from the path only if the proposal also deletes its unit. A
path containing any one source activates the target migration even when progress
recognition would require evidence for every source. All mappings in the
proposal read the same pre-publication target set, preventing recognitions
accepted together from feeding one another. Primary-key deduplication gives
splits, merges and overlapping rules their natural set semantics.

Progress uses the same durable unit identity and also records the accepted
proposal that was current when the completion was marked. Rebuilding or retiring
the current projection does not erase that record.
