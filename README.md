# simpanan.nvim 🗄️
Run any query by adding `|{your_connection}>` prefix!
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

# TODO video

### Data Pipelining
Grab a data from one database to another database. Useful if your project/s use microservice architecture with multiple databases.

# TODO video


## Requirements
- Go 1.20+
- Neovim 0.9+ (for `vim.json` and `vim.uv`)

## Installation

### lazy.nvim

```lua
{
  'adibfarrasy/simpanan.nvim',
  dependencies = { 'MunifTanjim/nui.nvim' },
  ft = 'simpanan',                       -- lazy-load on *.simp buffers
  cmd = { 'Simpanan' },                  -- …and on :Simpanan commands
  build = 'make -C simpanan',
  config = function()
    require('simpanan').setup {}
    vim.keymap.set('n', '<leader>sic', require('simpanan').list_connections)
    vim.keymap.set('v', '<leader>sie', require('simpanan').execute)
  end,
},
```

### Without a plugin manager

Clone the repo somewhere on your `runtimepath` (e.g. `~/.config/nvim/pack/simpanan/start/simpanan.nvim`),
run `make -C simpanan` once to build the Go backend and write
`~/.local/share/nvim/rplugin.vim`, then add to your `init.lua`:

```lua
require('simpanan').setup {}
vim.keymap.set('n', '<leader>sic', require('simpanan').list_connections)
vim.keymap.set('v', '<leader>sie', require('simpanan').execute)
```

After the first install (either method), run `:UpdateRemotePlugins`
once and restart Neovim so the Go remote-plugin manifest is picked up.

## Getting Started
1. Register your connections with `<leader>sic` → press `a` in the popup → type
   `label>uri` (e.g. `pg0>postgres://user:pass@localhost:5432/app`). Delete
   with `d`. (The popup's add form takes the bare `label>uri` form; the
   leading `|` is only used in the editor buffer itself.)
2. Open any `.simp` file (or just any buffer), write a query prefixed with
   `|label>` (e.g. `|pg0> SELECT * FROM users`), visually select it, and
   press `<leader>sie`.
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
- **Context-aware autocomplete** (via `nvim-cmp`) — connection labels at
  stage start, SQL keywords and alias-scoped columns, Mongo database /
  collection / operation / `$`-operator fields, Redis commands, jq
  operators, and jq paths probed on demand from the prior pipeline's
  output. Schema cache refreshes hourly; jq path probes cache for 30s.

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

### Autocomplete (nvim-cmp)

`require('simpanan').setup()` registers a `simpanan` source with
`nvim-cmp` (if installed). To surface its suggestions only in `.simp`
buffers (filetype `simpanan`), add a per-filetype source list:

```lua
local cmp = require('cmp')
-- after cmp.setup{...}:
cmp.setup.filetype('simpanan', {
  sources = cmp.config.sources({
    { name = 'simpanan' },
    { name = 'path' },
  }),
})
```

Triggers on `|`, `.`, `{`, `>`, `$` plus any word character, with an
80ms debounce. Suggestions only appear for connection labels you've
registered, since the classifier needs to know what type a label maps
to. A label that isn't registered silently produces no suggestions.

## FAQ
- What's with the name?
    - It means 'storage' in Bahasa Indonesia (I'm Indonesian). I thought it's apt.
