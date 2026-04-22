if exists("b:current_syntax")
  finish
endif

" Line comment: //...
syntax match simpananComment "^\s*//.*$"

" Connection prefix at start of a (trimmed) line: label>
" Captures labels like `pg0`, `my-db`, `jq`, etc. The `>` is part of the match.
syntax match simpananConnLabel "^\s*\S\+>" contains=simpananConnSep
syntax match simpananConnSep ">" contained

" jq placeholder: {{ ... }}
syntax region simpananPlaceholder start="{{" end="}}" contains=simpananPlaceholderDelim
syntax match  simpananPlaceholderDelim "{{\|}}" contained

" String literals (single- and double-quoted) — useful inside SQL/Mongo/Redis
" query text. Kept simple; does not attempt to be language-aware.
syntax region simpananString start=+"+ skip=+\\"+ end=+"+
syntax region simpananString start=+'+ skip=+\\'+ end=+'+

" Numbers
syntax match simpananNumber "\<\d\+\(\.\d\+\)\?\>"

" Fallback keywords for stages whose dialect we don't know (Mongo, Redis,
" or an SQL connection not yet registered). Stages whose label maps to a
" Postgres or MySQL connection are upgraded to full SQL syntax at runtime
" by ftplugin/simpanan.lua.
syntax case ignore
syntax keyword simpananKeyword
      \ show describe explain use
      \ find findOne aggregate count distinct
      \ insertOne insertMany updateOne updateMany deleteOne deleteMany
      \ get set del exists expire ttl keys hget hset lpush rpush
syntax case match

highlight default link simpananComment          Comment
highlight default link simpananConnLabel        Identifier
highlight default link simpananConnSep          Operator
highlight default link simpananPlaceholder      PreProc
highlight default link simpananPlaceholderDelim Delimiter
highlight default link simpananString           String
highlight default link simpananNumber           Number
highlight default link simpananKeyword          Keyword

let b:current_syntax = "simpanan"
