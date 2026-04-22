# simpanan.nvim 🗄️
Run any query by adding `{your_connection}>` prefix!
On any file, right on your editor!

The vision for this Neovim plugin is to:
- Create a convenient polyglot query runner for many popular databases, e.g.
  PostgreSQL, MySQL, MongoDB, and Redis.
- Remove the need for many database clients such as DBeaver for relational
  databases, MongoDB Compass for MongoDB, etc. - thus saving precious disk space
  and improve development speed from having to jump from one client to another.
- Standardize all database output into the de-facto standard JSON. This will
  enable data transformation (via [gojq](https://github.com/itchyny/gojq)) and pipelining from one database query to another.

## Showcase
### Basic Query
Add connection prefix to run query.

[simpanan.webm](https://github.com/adibfarrasy/simpanan.nvim/assets/28698955/f758b82b-b9d7-493d-8425-e64cfe2d952f)

### Data Pipelining
Grab a data from one database to another database. Useful if your project/s use microservice architecture with multiple databases.

[Screencast from 2024-04-21 15-26-20.webm](https://github.com/adibfarrasy/simpanan.nvim/assets/28698955/b5cd46e2-54bf-4bdd-9822-a8eee938f3a6)


## Requirements
- Go 1.20+
- Neovim 0.9+ (for `vim.json` and `vim.uv`)

## Installation
lazy.nvim:
```lua
{
  'adibfarrasy/simpanan.nvim',
  dependencies = { 'MunifTanjim/nui.nvim' },
  ft = 'simpanan',                     -- lazy-load on *.simp buffers
  cmd = { 'Simpanan' },                -- …and on :Simpanan commands
  build = 'make -C simpanan',
  config = function()
    require('simpanan').setup({})
    vim.keymap.set('n', '<leader>sc', require('simpanan').list_connections)
    vim.keymap.set('v', '<leader>se', require('simpanan').execute)
  end,
},
```

After the first install, run `:UpdateRemotePlugins` once and restart Neovim so the Go remote-plugin manifest is picked up.

## Getting Started
1. Register your connections with `<leader>sc` → press `a` in the popup → type
   `label>uri` (e.g. `pg0>postgres://user:pass@localhost:5432/app`). Delete
   with `d`.
2. Open any `.simp` file (or just any buffer), write a query prefixed with
   `label>`, visually select it, and press `<leader>se`.
3. Want a tour? See [`examples/`](./examples/) for annotated `.simp` files
   covering basic stages, pipelining with `{{jq}}` placeholders, and
   cross-database workflows.

## Features
- **Polyglot query execution** — Postgres (read / write / `\dt` / `\d`),
  MySQL (read / write), MongoDB (find, findOne, aggregate, distinct, count,
  insert/update/delete, show collections), Redis, and a built-in `jq>`
  transformer.
- **Data pipelining** — chain stages across connections, pass the previous
  stage's JSON into the next via `{{<jq-expression>}}` placeholders.
- **Connection management UI** — add, list, and delete connections without
  leaving the editor.
- **Custom `.simp` filetype** — syntax highlighting for stages, comments,
  and placeholders. Stages whose label resolves to a Postgres or MySQL
  connection get **full SQL highlighting** injected automatically from
  Neovim's bundled `sql.vim`. Highlighting refreshes live as connections
  are added or deleted.

## Configuration

```lua
require('simpanan').setup({
  max_row_limit = 20,    -- cap rows returned per read query (default 20)
  debug_mode    = false, -- when true, the output buffer shows each stage's
                         -- {conn_type, query, result} instead of only the
                         -- final result. Useful for diagnosing placeholder
                         -- substitution.
})
```

## FAQ
- What's with the name?
    - It means 'storage' in Bahasa Indonesia (I'm Indonesian). I thought it's apt.
