# Learning paths

Authenticated users can maintain several personal learning paths. A path stores
a name, an optional description and an ordered set of target unit IDs.

The default graph for a path is the induced subgraph containing its targets and
every published prerequisite that can lead to them. Focusing a unit applies the
same local-neighbourhood navigation used by the full curriculum, but never
escapes the path subgraph.

Learning paths reference stable projected unit IDs. Rebuilding the current
curriculum projection reconciles unit rows instead of deleting and recreating
them, so surviving path references remain valid across publications.
