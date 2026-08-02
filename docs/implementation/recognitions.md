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

`unit_completion_events` is an append-oriented history of direct completion and
return-to-pending actions. Every event records the accepted proposal that was
current at that moment. The latest event supplies the user's current direct
state, while earlier positive events remain available as historical evidence
for convalidation and certification. Accepted recognitions are followed
transitively when reading current progress. Derived recognition is never
persisted, so it cannot overwrite or obscure the evidence from which it came.

A direct completion remains literal while the unit's content is unchanged. A
later content update preserves it as recognized completion; explicit
recognition changes can copy completion to other unit identities in the same
way. Neither mechanism removes completion from its source. Marking a recognized
unit as completed records fresh direct evidence against the current accepted
proposal, without introducing a separate reaffirmation action.

The learning interface distinguishes a direct completion from recognition
derived from previous learning. A recognized unit cannot be independently
returned to pending while all of its source completions remain true.

## Publication warning

Recognition coverage is advisory rather than a proposal invariant. The
publishing UI asks for confirmation when a created unit has no incoming
recognition or a deleted unit has no outgoing recognition. The confirmation is
ephemeral UI state: the absence of recognition already records the resulting
curriculum semantics unambiguously.
