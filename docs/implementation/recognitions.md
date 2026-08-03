# Recognitions

## Proposal representation

A `recognition` proposal change contains one or more source unit IDs and one
or more target unit IDs. All sources belong to the proposal's base curriculum
and all targets belong to its resulting curriculum. The containing proposal's
rationale explains why the mapping belongs in the curriculum history; the
recognition itself records only the structured mapping.
The change is evaluated against those two complete states rather than applied as
an ordered graph mutation.

The recognition rule means that having completed every source before the
proposal is accepted recognizes every target. It is a temporal migration of
prior evidence rather than an equivalence that future source completions can
activate. This single rule supports one-to-one replacements, one-to-many splits
and many-to-one merges without percentages or partial-credit semantics.

## Progress

`unit_completions` stores at most one current direct completion per learner and
unit, together with the accepted proposal against which it was last completed.
Completing again updates that proposal; returning it to pending deletes the
direct row.

`unit_completion_recognitions` stores derived evidence keyed by learner,
target and recognition change. When a proposal is accepted, all of its rules
are evaluated against one materialized snapshot containing direct completions
and recognitions from earlier proposals. Their results are then inserted
without allowing rules accepted together to feed one another. A later proposal
can consume those persisted results, so transitive recognition proceeds only
forwards through curriculum history. The recognition change remains the
provenance of each derived completion without duplicating its source set. It is
a materialized projection rather than an irreversible fact. Returning either a
direct or recognized completion to pending removes that unit's direct and
derived rows, then repeatedly prunes downstream rows whose sources no longer
have evidence from an earlier proposal. This operation only removes rows; it
does not rematerialize a recognition and therefore supports a soft dismissal
without storing a permanent exclusion. An explicit historical catch-up or
projection rebuild may recognize the unit again later.

A direct completion remains literal while the unit's content is unchanged. A
later content update preserves it as recognized completion; explicit
recognition changes can copy completion to other unit identities in the same
way. Neither mechanism removes completion from its source. Marking a recognized
unit as completed records fresh direct evidence against the current accepted
proposal, without introducing a separate reaffirmation action.

The learning interface distinguishes a direct completion from recognition
derived from previous learning. Returning a directly completed unit to pending
rebuilds derived progress and may reveal another recognition route that remains
supported by the learner's other direct completions.

Future certification imports should record the curriculum proposal whose frozen
content they certify, independently of when the evidence reaches the platform.
The same propagation operation can then replay proposal groups from that point
to the current head. All recognitions in one proposal must still share the
evidence snapshot that preceded it.

## Publication warning

Recognition coverage is advisory rather than a proposal invariant. The
publishing UI asks for confirmation when a created unit has no incoming
recognition or a deleted unit has no outgoing recognition. The confirmation is
ephemeral UI state: the absence of recognition already records the resulting
curriculum semantics unambiguously.
