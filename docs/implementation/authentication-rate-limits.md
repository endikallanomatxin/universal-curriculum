# Authentication rate limits

Public authentication mutations use one persistent PostgreSQL-backed rate
limiter. Login, registration, password-reset requests and password-reset
attempts have independent per-IP scopes so abuse of one workflow does not deny
access to another.

The limit is recorded after a request body can be parsed but before password
hashing, email delivery or account creation. Login and reset attempts reject
the eleventh submission within 15 minutes; registration and recovery-email
requests reject the sixth. A rejected scope remains blocked for 15 minutes and
returns `429 Too Many Requests` with a `Retry-After` header.

Rate-limit rows are retained for at most 24 hours after they become inactive.
Client addresses come exclusively from `ClientIP`; forwarded headers are only
trusted when the deployment explicitly enables its Render proxy contract.
