-- Postgres schema + seed data for the simpanan examples.
-- Auto-runs on first container start because it lives in
-- /docker-entrypoint-initdb.d/.

-- ---- users -----------------------------------------------------

CREATE TABLE users (
    id          BIGSERIAL PRIMARY KEY,
    email       TEXT UNIQUE NOT NULL,
    name        TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'member',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO users (id, email, name, role, created_at) VALUES
    (1,    'alice@example.com', 'Alice',  'admin',  '2024-02-15 09:30:00+00'),
    (2,    'bob@example.com',   'Bob',    'member', '2024-04-02 14:11:00+00'),
    (3,    'carol@example.com', 'Carol',  'admin',  '2024-06-20 11:00:00+00'),
    (4,    'dave@example.com',  'Dave',   'member', '2026-01-10 08:00:00+00'),
    (5,    'erin@example.com',  'Erin',   'member', '2026-02-22 16:45:00+00'),
    (42,   'fox@example.com',   'Fox',    'member', '2025-11-01 12:00:00+00'),
    (1337, 'leet@example.com',  'Leet',   'member', '2025-09-14 20:20:00+00');

SELECT setval(pg_get_serial_sequence('users','id'),
              (SELECT MAX(id) FROM users));

-- ---- orders ---------------------------------------------------

CREATE TABLE orders (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    total      NUMERIC(10, 2) NOT NULL,
    status     TEXT NOT NULL DEFAULT 'pending',
    placed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO orders (user_id, total, status, placed_at) VALUES
    (1,  120.00, 'paid',    '2026-03-01 10:00:00+00'),
    (1,   45.50, 'paid',    '2026-03-15 13:45:00+00'),
    (2,   78.25, 'paid',    '2026-02-20 09:15:00+00'),
    (3,  210.00, 'paid',    '2026-01-05 18:00:00+00'),
    (3,   33.10, 'pending', '2026-03-29 22:00:00+00'),
    (4,    9.99, 'paid',    '2026-03-30 07:30:00+00');

-- ---- rewards + reward_balances --------------------------------

CREATE TABLE rewards (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT,
    points_cost INTEGER NOT NULL
);

INSERT INTO rewards VALUES
    ('1c1c0001-0000-4000-8000-000000000001'::UUID, 'Free coffee',     'A standard espresso on the house.', 100),
    ('1c1c0002-0000-4000-8000-000000000002'::UUID, '10% discount',    'One-time 10% off your next order.',  500),
    ('1c1c0003-0000-4000-8000-000000000003'::UUID, 'Branded mug',     'A simpanan-branded ceramic mug.',   1500);

CREATE TABLE reward_balances (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reward_id   UUID NOT NULL REFERENCES rewards(id),
    balance     INTEGER NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO reward_balances (user_id, reward_id, balance, created_at) VALUES
    (1, '1c1c0002-0000-4000-8000-000000000002'::UUID, 200, '2026-04-20 09:00:00+00'),
    (2, '1c1c0001-0000-4000-8000-000000000001'::UUID, 350, '2026-04-21 14:30:00+00'),
    (3, '1c1c0003-0000-4000-8000-000000000003'::UUID, 450, '2026-04-22 11:00:00+00'),
    (1, '1c1c0001-0000-4000-8000-000000000001'::UUID, 120, '2026-03-30 16:00:00+00');

-- ---- audit_log ------------------------------------------------

CREATE TABLE audit_log (
    id            BIGSERIAL PRIMARY KEY,
    actor_email   TEXT NOT NULL,
    action        TEXT NOT NULL,
    target        TEXT,
    ts            TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO audit_log (actor_email, action, target, ts) VALUES
    ('alice@example.com', 'login',           NULL,           '2026-04-22 08:00:00+00'),
    ('alice@example.com', 'role_changed',    'bob -> admin', '2026-04-22 09:15:00+00'),
    ('carol@example.com', 'reward_granted',  'rewards/3',    '2026-04-21 16:00:00+00'),
    ('alice@example.com', 'logout',          NULL,           '2026-04-22 18:30:00+00');

-- ---- products -------------------------------------------------

CREATE TABLE products (
    id           BIGSERIAL PRIMARY KEY,
    name         TEXT NOT NULL,
    price_cents  INTEGER NOT NULL
);

INSERT INTO products (id, name, price_cents) VALUES
    (1, 'Standard espresso',  300),
    (2, 'Cappuccino',         450),
    (3, 'Iced latte',         500),
    (4, 'Branded mug',       1800),
    (5, 'Beans (250g)',      1200);

SELECT setval(pg_get_serial_sequence('products','id'),
              (SELECT MAX(id) FROM products));
