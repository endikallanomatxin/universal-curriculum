# Learning paths

Authenticated users can maintain several personal learning paths. A path stores
a name and a set of target unit IDs.

The default graph for a path is the induced subgraph containing its targets and
every published prerequisite that can lead to them. Focusing a unit applies the
same local-neighbourhood navigation used by the full curriculum, but never
escapes the path subgraph.

Learning paths reference the stable ID of each unit's creation change rather
than the current `units` projection. Retiring a unit therefore preserves it as a
path target and marks it unavailable. A later creation receives a distinct
identity rather than silently inheriting the retired target.

Progress uses the same durable unit identity and also records the accepted
proposal that was current when the completion was marked. Rebuilding or retiring
the current projection does not erase that record.
