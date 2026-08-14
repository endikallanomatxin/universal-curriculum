-- +goose Up

ALTER TABLE users ADD COLUMN is_contributor BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE curriculum_proposals DROP CONSTRAINT curriculum_proposals_status_check;
ALTER TABLE curriculum_proposals
    ADD CONSTRAINT curriculum_proposals_status_check
    CHECK (status IN ('draft', 'submitted', 'accepted', 'rejected'));
ALTER TABLE curriculum_proposals
    ADD COLUMN submitted_at TIMESTAMPTZ,
    ADD COLUMN decided_at TIMESTAMPTZ,
    ADD COLUMN decided_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN rejection_reason TEXT CHECK (
        rejection_reason IS NULL OR
        (rejection_reason <> '' AND rejection_reason = btrim(rejection_reason))
    );
ALTER TABLE curriculum_proposals DROP CONSTRAINT curriculum_proposals_check1;
ALTER TABLE curriculum_proposals ADD CHECK (
    (status = 'draft' AND submitted_at IS NULL AND decided_at IS NULL AND decided_by IS NULL AND accepted_at IS NULL AND rejection_reason IS NULL)
    OR
    (status = 'submitted' AND submitted_at IS NOT NULL AND decided_at IS NULL AND decided_by IS NULL AND accepted_at IS NULL AND rejection_reason IS NULL)
    OR
    (status = 'accepted' AND submitted_at IS NOT NULL AND decided_at IS NOT NULL AND decided_by IS NOT NULL AND accepted_at = decided_at AND rejection_reason IS NULL)
    OR
    (status = 'rejected' AND submitted_at IS NOT NULL AND decided_at IS NOT NULL AND decided_by IS NOT NULL AND accepted_at IS NULL AND rejection_reason IS NOT NULL)
);

CREATE TABLE contributor_invitations (
    id BIGSERIAL PRIMARY KEY,
    email TEXT NOT NULL CHECK (
        email <> '' AND email = lower(email) AND email = btrim(email)
        AND char_length(email) <= 320
    ),
    token_hash TEXT NOT NULL UNIQUE,
    invited_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    accepted_by BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (expires_at > created_at),
    CHECK ((accepted_by IS NULL) = (accepted_at IS NULL)),
    CHECK (accepted_at IS NULL OR revoked_at IS NULL)
);

CREATE INDEX contributor_invitations_email_created_at_idx
    ON contributor_invitations (email, created_at DESC);
CREATE INDEX contributor_invitations_expires_at_idx
    ON contributor_invitations (expires_at)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- +goose Down

DROP TABLE contributor_invitations;
ALTER TABLE curriculum_proposals DROP CONSTRAINT curriculum_proposals_check1;
ALTER TABLE curriculum_proposals
    DROP COLUMN rejection_reason,
    DROP COLUMN decided_by,
    DROP COLUMN decided_at,
    DROP COLUMN submitted_at;
ALTER TABLE curriculum_proposals DROP CONSTRAINT curriculum_proposals_status_check;
ALTER TABLE curriculum_proposals
    ADD CONSTRAINT curriculum_proposals_status_check
    CHECK (status IN ('draft', 'accepted', 'rejected'));
ALTER TABLE curriculum_proposals ADD CHECK (
    (status = 'accepted' AND accepted_at IS NOT NULL)
    OR
    (status <> 'accepted' AND accepted_at IS NULL)
);
ALTER TABLE users DROP COLUMN is_contributor;
