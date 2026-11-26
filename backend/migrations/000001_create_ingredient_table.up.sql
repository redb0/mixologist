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

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TYPE recipe_status_enum AS ENUM ('draft', 'published');

CREATE TABLE recipes (
    id SERIAL PRIMARY KEY,
    author_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    servings INTEGER DEFAULT NULL,
    prep_time_minutes INTEGER DEFAULT NULL,
    status recipe_status_enum NOT NULL DEFAULT 'draft',
    published_at TIMESTAMPTZ DEFAULT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT recipes_author_id_fkey FOREIGN KEY (author_id) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT recipes_title_length_check CHECK (title ~ '\S' AND char_length(title) <= 200),
    CONSTRAINT recipes_description_length_check CHECK (char_length(description) <= 5000),
    CONSTRAINT recipes_servings_check CHECK (servings IS NULL OR servings > 0),
    CONSTRAINT recipes_prep_time_minutes_check CHECK (prep_time_minutes IS NULL OR prep_time_minutes >= 0),
    CONSTRAINT recipes_version_check CHECK (version > 0),
    CONSTRAINT recipes_published_at_check CHECK ((status = 'published') = (published_at IS NOT NULL))
);

-- Индекс используется для постраничного вывода публичного каталога рецептов
CREATE INDEX recipes_catalogue_keyset_idx ON recipes (published_at DESC, id DESC) WHERE status = 'published';

-- Индекс используется для постраничного вывода рецептов в админке
CREATE INDEX recipes_admin_keyset_idx ON recipes (updated_at DESC, id DESC);

-- Индекс используется для регистронезависимого поиска по подстроке названия (ILIKE)
CREATE INDEX recipes_title_trgm_idx ON recipes USING GIN (title gin_trgm_ops);

COMMENT ON TABLE recipes IS 'Рецепты коктейлей';
COMMENT ON COLUMN recipes.author_id IS 'Администратор, создавший рецепт';
COMMENT ON COLUMN recipes.title IS 'Название рецепта';
COMMENT ON COLUMN recipes.description IS 'Краткое описание рецепта (plain text)';
COMMENT ON COLUMN recipes.servings IS 'Количество порций';
COMMENT ON COLUMN recipes.prep_time_minutes IS 'Время приготовления в минутах';
COMMENT ON COLUMN recipes.status IS 'Статус публикации рецепта';
COMMENT ON COLUMN recipes.published_at IS 'Дата и время последней публикации';
COMMENT ON COLUMN recipes.version IS 'Версия рецепта';
COMMENT ON COLUMN recipes.created_at IS 'Дата и время создания рецепта';
COMMENT ON COLUMN recipes.updated_at IS 'Дата и время последнего обновления рецепта';

CREATE TABLE recipe_ingredients (
    recipe_id INTEGER NOT NULL,
    position INTEGER NOT NULL,
    ingredient_id INTEGER NOT NULL,
    amount NUMERIC(10, 2) NOT NULL,
    unit unit_measurement_enum NOT NULL,
    optional BOOLEAN NOT NULL DEFAULT FALSE,
    CONSTRAINT recipe_ingredients_pkey PRIMARY KEY (recipe_id, position),
    CONSTRAINT recipe_ingredients_ingredient_unique UNIQUE (recipe_id, ingredient_id),
    CONSTRAINT recipe_ingredients_recipe_id_fkey FOREIGN KEY (recipe_id) REFERENCES recipes (id) ON DELETE CASCADE,
    CONSTRAINT recipe_ingredients_ingredient_id_fkey FOREIGN KEY (ingredient_id) REFERENCES ingredients (id) ON DELETE RESTRICT,
    CONSTRAINT recipe_ingredients_position_check CHECK (position >= 0),
    CONSTRAINT recipe_ingredients_amount_check CHECK (amount > 0)
);

-- Индекс используется для проверки FK при удалении ингредиента и поиска рецептов по ингредиенту
CREATE INDEX recipe_ingredients_ingredient_id_idx ON recipe_ingredients (ingredient_id);

COMMENT ON TABLE recipe_ingredients IS 'Состав рецепта';
COMMENT ON COLUMN recipe_ingredients.recipe_id IS 'Рецепт';
COMMENT ON COLUMN recipe_ingredients.ingredient_id IS 'Ингредиент из каталога';
COMMENT ON COLUMN recipe_ingredients.position IS 'Порядок ингредиента в составе';
COMMENT ON COLUMN recipe_ingredients.amount IS 'Количество ингредиента';
COMMENT ON COLUMN recipe_ingredients.unit IS 'Единица измерения количества';
COMMENT ON COLUMN recipe_ingredients.optional IS 'Опциональный ингредиент';

CREATE TABLE recipe_instructions (
    recipe_id INTEGER NOT NULL,
    position INTEGER NOT NULL,
    markdown TEXT NOT NULL,
    CONSTRAINT recipe_instructions_pkey PRIMARY KEY (recipe_id, position),
    CONSTRAINT recipe_instructions_recipe_id_fkey FOREIGN KEY (recipe_id) REFERENCES recipes (id) ON DELETE CASCADE,
    CONSTRAINT recipe_instructions_position_check CHECK (position >= 0),
    CONSTRAINT recipe_instructions_markdown_length_check CHECK (markdown ~ '\S' AND char_length(markdown) <= 10000)
);

COMMENT ON TABLE recipe_instructions IS 'Шаги приготовления рецепта';
COMMENT ON COLUMN recipe_instructions.recipe_id IS 'Рецепт';
COMMENT ON COLUMN recipe_instructions.position IS 'Порядок шага';
COMMENT ON COLUMN recipe_instructions.markdown IS 'Текст шага в Markdown';
