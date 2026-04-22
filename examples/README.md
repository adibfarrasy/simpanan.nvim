# Examples

Three `.simp` files walking from "hello world" to a realistic polyglot
pipeline. Open any of them in Neovim after installing simpanan; with
matching connections registered (see `:Simpanan list_connections` → `a`),
`<leader>se` on a visual selection runs the selected stages.

1. **`01_basics.simp`** — what a stage is, comments, multi-line queries,
   Postgres `\d` admin shortcuts, Mongo/Redis/MySQL stages.
2. **`02_pipelining.simp`** — chaining stages with `{{jq}}` placeholders,
   cross-connection joins, the built-in `jq>` transformer.
3. **`03_complex.simp`** — realistic patterns: Mongo aggregation → jq
   reshape → SQL enrich; SQL cohort → jq array → Mongo `$in`; Redis
   counters cross-referenced with Postgres user data; debug-mode tip.

The example labels (`pg0`, `my0`, `mongo1`, `cache`) are arbitrary — use
your own labels when registering connections. Anything without a
registered label will fall back to generic highlighting and error at
execute time.
