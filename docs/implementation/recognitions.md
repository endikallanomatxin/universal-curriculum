# Recognitions

## Proposal representation

A `recognition` proposal change contains one or more source unit IDs, one
or more target unit IDs and a non-empty rationale. All sources belong to the
proposal's base curriculum and all targets belong to its resulting curriculum.
The change is evaluated against those two complete states rather than applied as
an ordered graph mutation.

The recognition rule means that completing every source recognizes every
target. This single rule supports one-to-one replacements, one-to-many splits
and many-to-one merges without percentages or partial-credit semantics.

## Progress

`completed_units` stores only direct user completions and the accepted proposal
that was current when each completion was recorded. Accepted recognitions are
followed transitively when reading progress. Derived recognition is never
inserted into `completed_units`, so it cannot overwrite or obscure the
historical evidence from which it came.

The learning interface distinguishes a direct completion from recognition
derived from previous learning. A recognized unit cannot be independently
returned to pending while all of its source completions remain true.

## Publication warning

Recognition coverage is advisory rather than a proposal invariant. The
publishing UI asks for confirmation when a created unit has no incoming
recognition or a deleted unit has no outgoing recognition. The confirmation is
ephemeral UI state: the absence of recognition already records the resulting
curriculum semantics unambiguously.
