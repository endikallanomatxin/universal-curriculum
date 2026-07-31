# Universal Curriculum product specification

## 1. Purpose

Universal Curriculum is a public platform for collaboratively building and
learning from a free, high-quality curriculum.

It combines:

- a coherent map of what can be learned and in which order;
- shared educational material that improves over time;
- a standard that educational institutions can use for certification.

The platform complements universities and teachers. It gives them common
infrastructure for combining their work, while allowing anyone to learn
independently and seek certification only when needed.

## 2. Goals

- Make high-quality tertiary education freely accessible worldwide.
- Let students learn asynchronously and at their own pace.
- Replace isolated course catalogues with a coherent curriculum.
- Accumulate the best explanations instead of repeating the same work.
- Let professors, experts and students collaborate to improve the curriculum
  openly.
- Keep learning independent from exams and certification.
- Let universities certify learning against a shared standard.
- React quickly to new knowledge, technology and social needs.

## 3. Principles

### Mastery before schedule

Time is variable; mastery is the expected outcome. Independent students are not
grouped by age, cohorts or fixed deadlines.

### One curriculum, many paths

The platform maintains one shared, evolving curriculum rather than a marketplace
of overlapping courses. Students can pursue different goals and valid paths
through the same knowledge graph.

### An optimised starting point, not a single teaching style

Adapting explanations to each student can substantially improve learning, but
before there is evidence about how they approach a topic, choosing among
equivalent explanations is largely arbitrary. If we build and continuously
improve one explanation that works well for most students, it provides a better
starting point.

### Learning before certification

Learning material and practice are open. Certification is optional, separate
and issued by an accountable institution after an appropriate assessment.
Developing learning and certification independently prevents the curriculum
from being constrained by what is easy to assess or shaped around a particular
exam. Material and practice are designed first for understanding; certification
evaluates that learning afterwards.

### Open participation with visible decisions

Anyone may inspect how the curriculum is built. Contributions become official
only through a public proposal, discussion and decision process.

### Global access

The curriculum is free to read and reuse.

## 4. Curriculum structure

### 4.1 Knowledge graph

The curriculum is a directed acyclic graph:

- **units** are nodes;
- **dependencies** are directed edges.

A dependency `A → B` means that A should be mastered before studying B.

The published graph must satisfy these rules:

- a unit cannot depend on itself;
- a direct dependency cannot be duplicated;
- cycles are forbidden;
- every dependency refers to existing units;
- removing a unit requires resolving every dependency in which it is a
  prerequisite.

### 4.2 Units

A unit is the smallest independently learnable part of the curriculum. It has:

- a stable identity;
- a name;
- dependencies;
- written notes;
- an explanatory video or equivalent guided explanation;
- exercises and solved examples;
- translations and supporting resources.

Units should be small enough to combine into different learning paths and
precise enough to be certified independently.

### 4.3 Groups

Groups could provide context to large areas of the graph. They may contain
units and other groups.

Whether groups should be explicit entities or derived from unit similarity
remains an open question.

### 4.4 Published curriculum and history

Students see the latest accepted curriculum by default. Work in progress never
silently changes the learning view.

Every published change retains its proposal, authors, discussion, decision and
previous state. Accepted proposals identify historical curriculum states so
that learning records and certifications keep their original meaning.

Accepted proposals may define recognitions between the curriculum state on
which they are based and the state they produce. A recognition may require
several source units and may recognize several target units, allowing
replacements, splits and merges to preserve the value of prior learning without
rewriting its historical record.

Every historical state can be exported in a documented machine-readable format.
Authorship, contribution terms and third-party licences are preserved.

## 5. Learning experience

### 5.1 Exploring

Anyone can, without an account:

- browse the graph;
- search for units;
- inspect prerequisites and possible next steps;
- read notes and use educational resources;
- view a manageable local region instead of the entire graph at once.

### 5.2 Guidance

Students choose one or more target units. The platform:

1. finds every required dependency;
2. excludes units the student has already completed from the suggested path;
3. highlights valid paths to the goal;
4. recommends available next units;
5. explains why each unit is recommended.

When several paths are valid, the system presents the alternatives rather than
inventing a mandatory sequence.

Students can inspect each personal learning path separately or combine all of
their paths into one graph of targets and required prerequisites.

### 5.3 Progress

A student can mark a unit as completed or return it to pending. Completion is
private by default and remains distinct from certification.

Sharing progress or exercise submissions with a teacher or institution requires
explicit consent.

Curriculum changes must not erase or reinterpret historical progress. A
completion records the unit and accepted curriculum state in which it occurred.
The platform applies the recognitions in accepted proposals to show which
units in the current graph are also recognized. The original completion remains
distinct from the derived recognition.

### 5.4 Employment-informed guidance

Employers may describe opportunities through the units they require. Students
can either:

- select an opportunity and see the path towards its requirements; or
- follow their own interests while seeing which choices open more opportunities.

Labour-market information guides students but does not determine curriculum
content or override their goals.

### 5.5 Accounts

Anyone may create an account with their name, email address and a password.
Email addresses identify local accounts case-insensitively and cannot be shared
by multiple accounts. Public registration creates a regular member account and
never grants administrative access.

## 6. Exercises and feedback

Exercises help students verify understanding. They are not certification exams
and have no inherent deadlines. Exercise submissions are private by default.

### 6.1 Unsupervised exercises

Exercises with an unambiguous result should be corrected automatically whenever
this preserves their educational value. More complex procedural exercises may
use worked examples and reference solutions for self-assessment.

### 6.2 Supervised exercises

Creative or open-ended work requires human judgement. A submission may receive
several independent corrections. Reviewers can assess existing corrections and
submit alternatives.

The process ends when enough agreement exists. The student receives the
combined feedback.

Reviewer history produces area-specific reliability signals. These signals help
assign and weight corrections.

Students may review peer work when their demonstrated reliability makes this
safe. Teachers may also create closed groups in which they alone review their
students' work.

### 6.3 AI-assisted correction

AI may assist reviewers and propose corrections where evidence shows that it is
safe and useful.

## 7. Collaborative curriculum development

Anyone can inspect the process. Creating proposals, commenting and voting
require an authenticated account.

### 7.1 Proposals

A proposal is a coherent collection of changes. It may change:

- units;
- graph dependencies;
- how knowledge from earlier units is recognized in the resulting curriculum;
- notes, videos and other resources;
- exercises and reference solutions;
- translations.

A proposal contains its authors, rationale, the accepted proposal on which it
is based and a readable diff of both content and graph changes.

A recognition marks every target as recognized when all of its sources have
been completed. Recognitions are explicit proposal changes and may be proposed
even when no unit is created or removed. Creating a unit without incoming
recognition means that it represents knowledge not previously covered. Removing
a unit without outgoing recognition means that it has no recognized successor.
These choices are valid, but the publishing interface warns the author before
continuing so they are not made accidentally.

### 7.2 Isolated work and conflicts

Starting a proposal freezes its base curriculum. Its changes remain isolated
until accepted.

If the published curriculum changes meanwhile, the platform identifies
conflicts. Authors can incorporate the current accepted changes and resolve
those conflicts before submitting the final proposal for a decision.

### 7.3 Discussion

Every submitted proposal has a public discussion. Comments support replies and
upvotes or downvotes so useful contributions can surface, but comment popularity
does not decide the proposal.

### 7.4 Single-proposal polls

A proposal with no competing alternative is put to a vote against keeping the
current curriculum unchanged.

Polls do not close merely because one side is temporarily ahead. Their stopping
rule combines:

- a defined level of confidence that support exceeds the required threshold;
- a minimum number of votes;
- a minimum discussion period.

There may be stricter thresholds for larger or riskier changes.

### 7.5 Competing proposals

When open proposals modify the same part of the curriculum incompatibly, they
are decided together. The current curriculum remains an option.

Voters can express preferences between the viable alternatives, allowing each
proposal to be compared with both the current state and its competitors.

A minor, clearly beneficial revision may replace an option without restarting
the entire process.

### 7.6 Voting weight and integrity

Participation is open, but the system may give greater influence to demonstrated
knowledge relevant to the decision. Relevant evidence may include:

- units read or certified in the affected area;
- dependent units read or certified;
- accepted contributions;
- reliable reviewing history.

Weighting rules must be public, contestable and designed to avoid permanent
concentration of power.

The platform must resist bots, duplicate identities and coordinated
manipulation. Stronger identity verification may be required for consequential
decisions, but it must remain proportionate and protect participants' privacy.

### 7.7 Acceptance

An accepted proposal is validated against the current graph and applied
atomically. Its units, dependencies, recognitions, resources,
translations and decision record are either published together or not published
at all.

## 8. Languages

The curriculum has a canonical English source. Other languages are generated
from it and remain linked to the same units and graph.

Machine translation provides initial coverage. Human contributors can correct
translations where general translation fails, including terminology and
context-specific phrasing.

Translation models may learn from approved discrepancies.

## 9. Universities and certification

Universities remain the primary certifying institutions. The platform defines a
shared curriculum reference and records what an institution has certified.
Issuing a certification requires a verified institutional identity and
authority.

Institutions can participate at several levels:

- professors use units as primary or supplementary material;
- teachers manage closed student groups and exercises;
- a university adopts selected units or paths;
- a university records certifications after exams, projects or other assessment;
- a university offers certification to independent external students.

A certification identifies the student, institution, units, accepted curriculum
state, assessment basis, issue date, validity and revocation status.

Recognitions may show that an older certification covers units in the
current curriculum, but they never alter or replace the certification that was
issued. Issuing a new certification or a formal convalidation remains an
explicit decision of an authorized institution.

Because units are fine-grained, project-based learning can also be certified: an
institution evaluates a project and records the units whose knowledge was
successfully demonstrated.

## 10. Roles

- **Visitor:** reads the published curriculum and its public history.
- **Student:** chooses goals, studies, practises and records progress.
- **Contributor:** prepares proposals and joins discussions and polls.
- **Teacher or reviewer:** creates material and reviews open-ended work.
- **Certifying institution:** assesses students and issues certifications.
- **Moderator:** handles abuse without unilaterally deciding academic truth.

A person may perform multiple individual roles from the same account.

## 11. Stewardship

The platform is stewarded by a mission-driven, non-profit organisation. It is
responsible for operation, moderation, initial coordination and the
integrity of the public process, but it does not own academic truth.

Its governance, funding and conflicts of interest are public. Curriculum content
and contributions use explicit open licences, and exports allow the project to
survive a change of operator. No university, company, sponsor or infrastructure
provider can privately control the shared curriculum.
