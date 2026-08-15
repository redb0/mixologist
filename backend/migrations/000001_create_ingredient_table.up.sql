CREATE TYPE unit_measurement_enum AS ENUM ('мл', 'гр', 'шт', 'дэш');
CREATE TYPE abv_enum AS ENUM ('безалкогольный', 'слабоалкогольный', 'крепкий');
CREATE TYPE ingredient_type_enum AS ENUM ('крепкая часть', 'безалкогольная часть', 'вермут', 'вино', 'ликер', 'биттер', 'сироп', 'другое', 'фрукт', 'овощ', 'ягода');

CREATE TABLE ingredients (
    id SERIAL PRIMARY KEY,
    name VARCHAR(512) NOT NULL,
    description TEXT DEFAULT '',
    unit_measurement unit_measurement_enum NOT NULL,
    abv abv_enum NOT NULL,
    ingredient_type ingredient_type_enum NOT NULL DEFAULT 'другое',
    icon BYTEA DEFAULT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX ingredients_name_lower_unique ON ingredients (LOWER(name));

-- Индекс используется для постраничного вывода ингредиентов по умолчанию
CREATE INDEX ingredients_default_keyset_idx ON ingredients (created_at DESC, id DESC);

-- Индекс используется для сортировки ингредиентов по имени
CREATE INDEX ingredients_name_sort_idx ON ingredients (LOWER(name), id);

-- Индекс используется для фильтрации ингредиентов по типу и крепости
CREATE INDEX ingredients_type_abv_idx ON ingredients (ingredient_type, abv);

COMMENT ON TABLE ingredients IS 'Ингредиенты коктейлей';
COMMENT ON COLUMN ingredients.name IS 'Имя ингредиента';
COMMENT ON COLUMN ingredients.description IS 'Описание ингредиента';
COMMENT ON COLUMN ingredients.unit_measurement IS 'Единица измерения ингредиента';
COMMENT ON COLUMN ingredients.abv IS 'Крепость ингредиента';
COMMENT ON COLUMN ingredients.ingredient_type IS 'Тип ингредиента';
COMMENT ON COLUMN ingredients.icon IS 'Иконка ингредиента';
COMMENT ON COLUMN ingredients.version IS 'Версия ингредиента';
COMMENT ON COLUMN ingredients.created_at IS 'Дата и время создания ингредиента';
COMMENT ON COLUMN ingredients.updated_at IS 'Дата и время последнего обновления ингредиента';

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    google_subject VARCHAR(255) NOT NULL,
    email VARCHAR(320) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    avatar_url TEXT NOT NULL DEFAULT '',
    role VARCHAR(32) NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX users_google_subject_unique ON users (google_subject);

-- Учитываем регистр email для запрета дублей учетных записей.
CREATE UNIQUE INDEX users_email_lower_unique ON users (LOWER(email));

CREATE TABLE sessions (
    id SERIAL PRIMARY KEY,
    token_hash VARCHAR(255) NOT NULL,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ DEFAULT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX sessions_token_hash_unique ON sessions (token_hash);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);
CREATE INDEX sessions_revoked_at_idx ON sessions (revoked_at) WHERE revoked_at IS NOT NULL;
