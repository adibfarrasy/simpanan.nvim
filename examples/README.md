# Examples

Three `.simp` files walking from "hello world" to a realistic polyglot
pipeline. Open any of them in Neovim (or the webui) after installing
simpanan; with matching connections registered, `<leader>sie` on a
visual selection runs the selected stages.

1. **`01_basics.simp`** — what a stage is, comments, multi-line queries,
   Postgres `\d` admin shortcuts, Mongo/Redis/MySQL stages.
2. **`02_pipelining.simp`** — chaining stages with `{{jq}}` placeholders,
   cross-connection joins, the built-in `jq>` transformer.
3. **`03_complex.simp`** — realistic patterns: Mongo aggregation → jq
   reshape → SQL enrich; SQL cohort → jq array → Mongo `$in`; Redis
   counters cross-referenced with Postgres user data; debug-mode tip.

## Try it locally with Docker

A `docker-compose.yaml` in this directory spins up Postgres, MySQL,
MongoDB, and Redis on standard ports and pre-seeds them with the
schema + data the examples expect.

```sh
docker compose -f examples/docker-compose.yaml up -d
```

Wait a few seconds for the seed scripts to finish (you can `docker
compose -f examples/docker-compose.yaml logs redis-seed` to confirm
the seeder exited cleanly), then register the matching connections
in simpanan via the Connections popup or `:lua require('simpanan').list_connections()`:

| Label    | URI                                             |
|----------|-------------------------------------------------|
| `pg0`    | `postgres://simp:simp@localhost:5432/simp`      |
| `my0`    | `mysql://simp:simp@localhost:3306/simp`         |
| `mongo1` | `mongodb://localhost:27017/simp`                |
| `cache`  | `redis://localhost:6379`                        |

The example files reference exactly these labels, so once registered
every `|<label>>` selection in the example `.simp` files should run
end-to-end.

To tear everything down (and discard the seed data):

```sh
docker compose -f examples/docker-compose.yaml down -v
```

The `-v` flag also removes the named volumes, so the next `up -d`
will re-run the seed scripts from scratch.

## What's in the seed

- **Postgres** (`pg0` → `simp` database):
  `users`, `orders`, `reward_balances`, `rewards`, `audit_log`, `products`.
- **MySQL** (`my0` → `simp` database):
  `users`, `sessions`.
- **MongoDB** (`mongo1` → `simp` database):
  `orders`, `activity`.
- **Redis** (`cache`):
  string keys (`user:42`, `user:42:profile`, `ratelimit:user:1337`,
  `cache:product:*`), hash keys (`session:*`).

## Using your own connections

Labels (`pg0`, `my0`, `mongo1`, `cache`) are arbitrary — use any names
you like when registering connections, then either rewrite the example
files to match, or temporarily rename your labels to align. A label that
isn't registered silently produces no autocomplete suggestions and
errors at execute time with `Connection key '...' not found.`
