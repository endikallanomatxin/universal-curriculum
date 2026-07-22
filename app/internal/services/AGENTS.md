# Services

- Put business workflows, cross-entity validation and transaction coordination
  in this package.
- Do not add services that merely forward arguments to one database function.
- Return domain models and explicit errors that handlers can translate.
- Keep database mutations belonging to one workflow atomic.
- Add interfaces only for multiple implementations or a real test boundary.
- File-consuming workflows depend on `ObjectStore`, never on filesystem paths.
- Treat storage keys as opaque identifiers and keep backend-specific behaviour
  inside the storage implementation.
