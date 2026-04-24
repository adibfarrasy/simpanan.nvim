-- MySQL schema + seed data for the simpanan examples.
-- Auto-runs on first container start because it lives in
-- /docker-entrypoint-initdb.d/.

USE simp;

CREATE TABLE users (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    email       VARCHAR(255) NOT NULL UNIQUE,
    name        VARCHAR(120) NOT NULL,
    role        VARCHAR(32)  NOT NULL DEFAULT 'member',
    created_at  TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE = InnoDB;

INSERT INTO users (id, email, name, role, created_at) VALUES
    (1,  'alice@example.com', 'Alice', 'admin',  '2024-02-15 09:30:00'),
    (2,  'bob@example.com',   'Bob',   'member', '2024-04-02 14:11:00'),
    (3,  'carol@example.com', 'Carol', 'admin',  '2024-06-20 11:00:00'),
    (4,  'dave@example.com',  'Dave',  'member', '2026-01-10 08:00:00');

-- A second table so `SHOW TABLES` returns more than one entry.
CREATE TABLE sessions (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    user_id     BIGINT UNSIGNED NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX (user_id)
) ENGINE = InnoDB;

INSERT INTO sessions (user_id, created_at) VALUES
    (1, '2026-04-22 08:00:00'),
    (2, '2026-04-22 12:30:00'),
    (3, '2026-04-22 16:45:00');
