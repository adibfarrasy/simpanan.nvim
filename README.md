# simpanan.nvim 🗄️
Run any query by adding `{your_connection}>` prefix! 
On any file, right on your editor!

![Screenshot](assets/simpanan.webm)

The vision for this Neovim plugin is to:
- Create a convenient datastore runner for many popular databases, e.g. 
  PostgeSQL, MySQL, SQLite, MongoDB, and Redis.
- Remove the need for many database clients such as DBeaver for relational
  databases, MongoDB Compass for MongoDB, etc. - thus saving precious disk space
  and improve development speed from having to jump from one client to another.
- Standardize all database output into the de-facto standard JSON. This will
  enable pipelining from one database query to another.
- Did I mention data pipelining? Those are in the works too. NOTE: it's still in
  the architecture design stage, so it will not come very soon. Feel free to
  submit PRs to help!

## Requirements
- Go 1.20+

## Installation
- lazy.nvim:
```lua
  {
    dir = 'adibfarrasy/simpanan.nvim',
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
- Support for postgres query
- List connections

## WIP
These planned featueres will be supported in order:
1. Manage connections (add/ delete) from the list_connections popup
2. Support for mongodb query
3. Support for redis query
4. Support for MySQL and SQLite query
5. Data pipelining

## FAQ
- What's with the name?
    - It means 'storage' in Bahasa Indonesia (I'm Indonesian). I thought it's apt.
