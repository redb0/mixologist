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
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (name)
);

COMMENT ON TABLE ingredients IS 'Ингредиенты коктейлей';
COMMENT ON COLUMN ingredients.name IS 'Имя ингредиента';
COMMENT ON COLUMN ingredients.description IS 'Описание ингредиента';
COMMENT ON COLUMN ingredients.unit_measurement IS 'Единица измерения ингредиента';
COMMENT ON COLUMN ingredients.abv IS 'Крепость ингредиента';
COMMENT ON COLUMN ingredients.ingredient_type IS 'Тип ингредиента';
COMMENT ON COLUMN ingredients.icon IS 'Иконка ингредиента';
COMMENT ON COLUMN ingredients.created_at IS 'Дата и время создания ингредиента';
