DROP INDEX IF EXISTS sessions_revoked_at_idx;
DROP INDEX IF EXISTS sessions_expires_at_idx;
DROP INDEX IF EXISTS sessions_user_id_idx;
DROP INDEX IF EXISTS sessions_token_hash_unique;
DROP TABLE IF EXISTS sessions;

DROP INDEX IF EXISTS users_email_lower_unique;
DROP INDEX IF EXISTS users_google_subject_unique;
DROP TABLE IF EXISTS users;

DROP TABLE IF EXISTS ingredients;

DROP TYPE unit_measurement_enum;
DROP TYPE abv_enum;
DROP TYPE ingredient_type_enum;
