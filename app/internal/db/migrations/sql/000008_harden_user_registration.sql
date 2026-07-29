-- +goose Up
ALTER TABLE users
    ADD CONSTRAINT users_full_name_trimmed
        CHECK (full_name = btrim(full_name)),
    ADD CONSTRAINT users_full_name_length
        CHECK (char_length(full_name) <= 200);

ALTER TABLE local_authentications
    ADD CONSTRAINT local_authentications_email_trimmed
        CHECK (email = btrim(email)),
    ADD CONSTRAINT local_authentications_email_length
        CHECK (char_length(email) <= 320);

-- +goose Down
ALTER TABLE local_authentications
    DROP CONSTRAINT local_authentications_email_length,
    DROP CONSTRAINT local_authentications_email_trimmed;

ALTER TABLE users
    DROP CONSTRAINT users_full_name_length,
    DROP CONSTRAINT users_full_name_trimmed;
