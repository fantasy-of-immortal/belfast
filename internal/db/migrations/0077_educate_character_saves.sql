-- Legacy commander_tbs is retained as a read-only migration source.
CREATE TABLE commander_educate_characters (
 commander_id bigint NOT NULL REFERENCES commanders(commander_id) ON DELETE CASCADE,
 character_id bigint NOT NULL CHECK (character_id > 0),
 state bytea NOT NULL,
 permanent bytea NOT NULL,
 metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
 revision bigint NOT NULL DEFAULT 1,
 PRIMARY KEY (commander_id, character_id)
);
CREATE TABLE commander_educate_migrations (
 commander_id bigint PRIMARY KEY REFERENCES commanders(commander_id) ON DELETE CASCADE,
 original_state bytea NOT NULL,
 original_permanent bytea NOT NULL,
 report jsonb NOT NULL,
 applied_at timestamptz NOT NULL DEFAULT now()
);
