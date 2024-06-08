# simpanan.nvim 🗄️
Run any query by adding `{your_connection}>` prefix! 
On any file, right on your editor!

The vision for this Neovim plugin is to:
- Create a convenient polyglot query runner for many popular databases, e.g. 
  PostgeSQL, MySQL, SQLite, MongoDB, and Redis.
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

## Installation
- lazy.nvim:
```lua
  {
    'adibfarrasy/simpanan.nvim',
    dependencies = {
      'MunifTanjim/nui.nvim',
    },
    build = 'make -C simpanan',
    config = function()
      vim.keymap.set('n', '<leader>sc', require('simpanan').list_connections)
      vim.keymap.set('v', '<leader>se', require('simpanan').execute)
    end,
  },
```

## Getting Started
1. Add your connections to the connections JSON file located in
   `~/.local/share/nvim` (NOTE: currently this process has to be completely
   manual, the functionality to manage connections coming soon!)
2. If you use the config in the (Installation)[##Installation] section, you can
   start writing your query and execute it with the execute keymap.

## Features
- Support for PostgreSQL and some MongoDB read query
- List connections
- Data pipelining

## FAQ
- What's with the name?
    - It means 'storage' in Bahasa Indonesia (I'm Indonesian). I thought it's apt.
