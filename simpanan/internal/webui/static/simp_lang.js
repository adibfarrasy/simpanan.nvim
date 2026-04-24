// Webui-side syntax highlighter for .simp files.
//
// Implemented as a CodeMirror StreamLanguage rather than a Lezer
// grammar to avoid a build step. Limitation: it's a tokenizer, not a
// real parser — strong enough for keyword/string/number/comment
// colouring but won't catch grammar errors.
//
// Cross-stage context:
//   - Each line is checked for a `<label>>` header. If present, the
//     parser looks up the label in the live connection-types map
//     (passed in by the caller, kept up-to-date as the user adds /
//     deletes connections) and switches the per-stage tokenisation
//     mode for the rest of the line and any continuation lines.
//   - The reserved `jq` label always means jq.
//   - {{ ... }} placeholders within a non-jq stage body switch into a
//     jq sub-mode until the matching }}.

import { StreamLanguage } from "https://esm.sh/@codemirror/language@6.10.1";

// ---- Per-language keyword sets ------------------------------------

const SQL_KEYWORDS = new Set([
	"select", "from", "where", "and", "or", "not", "in", "like", "ilike",
	"is", "null", "between", "exists", "all", "any", "some",
	"join", "left", "right", "inner", "outer", "full", "cross", "on", "using",
	"group", "by", "having", "order", "asc", "desc", "limit", "offset",
	"union", "intersect", "except", "distinct", "as",
	"insert", "into", "values", "update", "set", "delete",
	"create", "table", "index", "view", "drop", "alter", "truncate",
	"with", "case", "when", "then", "else", "end", "returning",
	"count", "sum", "avg", "min", "max", "coalesce", "nullif",
	"true", "false",
	// MySQL extras (harmless if used in pg)
	"show", "databases", "tables", "columns", "describe", "explain", "use",
]);

const REDIS_COMMANDS = new Set([
	"get", "set", "setex", "setnx", "mget", "mset", "append", "getset",
	"incr", "incrby", "incrbyfloat", "decr", "decrby", "strlen",
	"del", "unlink", "exists", "type", "ttl", "pttl",
	"expire", "pexpire", "expireat", "pexpireat", "persist",
	"keys", "scan", "rename", "renamenx", "copy",
	"hget", "hset", "hsetnx", "hmget", "hmset", "hgetall",
	"hkeys", "hvals", "hlen", "hexists", "hdel", "hincrby",
	"lpush", "rpush", "lpop", "rpop", "lrange", "llen",
	"lindex", "lset", "lrem", "ltrim", "linsert",
	"sadd", "srem", "smembers", "sismember", "scard", "spop",
	"zadd", "zrem", "zrange", "zrevrange", "zrangebyscore",
	"zscore", "zcard", "zcount", "zrank", "zrevrank",
	"ping", "echo", "info", "time", "dbsize",
	"flushdb", "flushall", "select",
	"publish", "subscribe", "eval", "evalsha",
	"xadd", "xread", "xrange", "xlen", "xdel",
]);

const MONGO_COLLECTION_OPS = new Set([
	"find", "findone", "aggregate", "distinct", "count",
	"countdocuments", "estimateddocumentcount",
	"insertone", "insertmany", "updateone", "updatemany",
	"replaceone", "deleteone", "deletemany",
	"findoneandupdate", "findoneandreplace", "findoneanddelete",
	"bulkwrite", "createindex", "dropindex", "getindexes",
]);

const JQ_FUNCS = new Set([
	"select", "map", "map_values", "length", "keys", "keys_unsorted",
	"values", "has", "in", "contains", "type", "not",
	"to_entries", "from_entries", "with_entries",
	"add", "any", "all", "empty", "error", "range",
	"floor", "ceil", "round", "sqrt", "tonumber", "tostring",
	"split", "join", "ltrimstr", "rtrimstr", "startswith", "endswith",
	"test", "match", "capture", "scan", "splits", "sub", "gsub",
	"unique", "unique_by", "group_by", "sort", "sort_by",
	"reverse", "first", "last", "nth", "flatten", "reduce", "foreach",
	"recurse", "walk", "fromjson", "tojson",
	"if", "then", "else", "elif", "end", "as", "and", "or",
]);

// ---- Helpers ------------------------------------------------------

function langForLabel(label, connTypes) {
	if (label === "jq") return "jq";
	const ct = connTypes ? connTypes.get(label) : null;
	switch (ct) {
		case "postgres":
		case "mysql":
			return "sql";
		case "mongo":
			return "mongo";
		case "redis":
			return "redis";
		case "jq":
			return "jq";
	}
	return null;
}

function tokenForWord(word, lang) {
	if (!lang) return null;
	const lower = word.toLowerCase();
	if (lang === "sql" && SQL_KEYWORDS.has(lower)) return "keyword";
	if (lang === "redis" && REDIS_COMMANDS.has(lower)) return "keyword";
	if (lang === "mongo") {
		if (MONGO_COLLECTION_OPS.has(lower)) return "keyword";
		if (word.startsWith("$")) return "propertyName"; // $match, $group, …
	}
	if (lang === "jq" && JQ_FUNCS.has(lower)) return "keyword";
	return null;
}

// ---- Parser ------------------------------------------------------

function makeStartState() {
	return {
		stageLang: null,    // current stage's body language
		inPlaceholder: false, // inside {{ ... }} → tokenize as jq
		stringQuote: null,  // current string delimiter, null if not in a string
	};
}

function copyState(state) {
	return {
		stageLang: state.stageLang,
		inPlaceholder: state.inPlaceholder,
		stringQuote: state.stringQuote,
	};
}

function makeSimpParser(connTypesRef) {
	return {
		startState: makeStartState,
		copyState,
		token(stream, state) {
			// Inside a multi-line string, eat until the closing quote.
			if (state.stringQuote) {
				while (!stream.eol()) {
					const ch = stream.next();
					if (ch === "\\" && !stream.eol()) {
						stream.next();
						continue;
					}
					if (ch === state.stringQuote) {
						state.stringQuote = null;
						return "string";
					}
				}
				return "string";
			}

			if (stream.sol()) {
				// Line comment
				if (stream.match(/^\s*\/\//)) {
					stream.skipToEnd();
					return "comment";
				}
				// Stage header: "|<label>>" possibly with whitespace
				const m = stream.match(/^\|([^\s>|]+)\s*>/);
				if (m) {
					state.stageLang = langForLabel(m[1], connTypesRef.value);
					return "labelName";
				}
				// Continuation line — fall through to body tokenisation.
			}

			if (stream.eatSpace()) return null;

			// {{ ... }} placeholder; jump to / leave jq sub-mode.
			if (stream.match("{{")) { state.inPlaceholder = true; return "operator"; }
			if (state.inPlaceholder && stream.match("}}")) {
				state.inPlaceholder = false;
				return "operator";
			}

			const lang = state.inPlaceholder ? "jq" : state.stageLang;

			// String literal — opens; close handled at the top of the next
			// token call so multi-line strings work.
			const quote = stream.peek();
			if (quote === '"' || quote === "'") {
				stream.next();
				state.stringQuote = quote;
				while (!stream.eol()) {
					const ch = stream.next();
					if (ch === "\\" && !stream.eol()) { stream.next(); continue; }
					if (ch === quote) { state.stringQuote = null; return "string"; }
				}
				return "string";
			}

			// Number
			if (stream.match(/^-?\d+(?:\.\d+)?/)) return "number";

			// Backslash command (psql admin like \dt) at SQL stages.
			if (lang === "sql" && stream.match(/^\\[a-zA-Z]+/)) return "atom";

			// Word — identifier or keyword
			if (stream.match(/^[A-Za-z_$][A-Za-z0-9_$]*/)) {
				const tok = tokenForWord(stream.current(), lang);
				return tok;
			}

			// Punctuation we want to colour as operator (mostly inside jq)
			if (stream.match(/^[|\.\[\]:,()=]/)) {
				return lang === "jq" ? "operator" : null;
			}

			stream.next();
			return null;
		},
	};
}

// connTypesRef is { value: Map<label, "postgres"|"mysql"|"mongo"|"redis"|"jq"> }
// passed as a live reference so the parser always sees the up-to-date
// registry without needing to be reconstructed on every change.
export function simpStreamLanguage(connTypesRef) {
	return StreamLanguage.define(makeSimpParser(connTypesRef));
}
