package internal

import "simpanan/internal/common"

// Suggestion is a single autocomplete candidate returned to the editor.
type Suggestion struct {
	Text string         `json:"text"`
	Kind SuggestionKind `json:"kind"`
}

// SuggestionKind tags what a suggestion represents so the editor can
// render or filter appropriately.
type SuggestionKind string

const (
	SuggestionConnectionLabel SuggestionKind = "connection_label"
	SuggestionSqlKeyword      SuggestionKind = "sql_keyword"
	SuggestionDatabase        SuggestionKind = "database"
	SuggestionTable           SuggestionKind = "table"
	SuggestionColumn          SuggestionKind = "column"
	SuggestionMongoCollection SuggestionKind = "mongo_collection"
	SuggestionMongoOperation  SuggestionKind = "mongo_operation"
	SuggestionMongoOperator   SuggestionKind = "mongo_operator"
	SuggestionMongoField      SuggestionKind = "mongo_field"
	SuggestionRedisCommand    SuggestionKind = "redis_command"
	SuggestionJqOperator      SuggestionKind = "jq_operator"
	SuggestionJqPath          SuggestionKind = "jq_path"
)

// BuiltinCatalog is the static, ship-with-the-plugin completion knowledge
// for one connection type. Contents are hand-maintained rather than
// introspected from the backing database.
type BuiltinCatalog struct {
	ConnectionType            common.ConnType
	SqlKeywords               []string
	RedisCommands             []string
	MongoAggregationOperators []string
	MongoCollectionOperations []string
	JqOperators               []string
}

// GetBuiltinCatalog returns the static catalog for the given connection
// type. Unknown types yield an empty catalog rather than an error so
// callers can treat it uniformly.
func GetBuiltinCatalog(ct common.ConnType) BuiltinCatalog {
	switch ct {
	case common.Postgres:
		return postgresCatalog
	case common.Mysql:
		return mysqlCatalog
	case common.Mongo:
		return mongoCatalog
	case common.Redis:
		return redisCatalog
	case common.Jq:
		return jqCatalog
	}
	return BuiltinCatalog{ConnectionType: ct}
}

// sqlKeywordsCommon is the cross-dialect SQL keyword set. Each dialect
// catalog extends it with engine-specific keywords (e.g. PostgreSQL
// psql meta-commands, MySQL SHOW variants).
var sqlKeywordsCommon = []string{
	"SELECT", "FROM", "WHERE", "AND", "OR", "NOT", "IN", "LIKE", "ILIKE",
	"IS", "NULL", "BETWEEN", "EXISTS", "ALL", "ANY", "SOME",
	"JOIN", "LEFT", "RIGHT", "INNER", "OUTER", "FULL", "CROSS", "ON", "USING",
	"GROUP", "BY", "HAVING", "ORDER", "ASC", "DESC", "LIMIT", "OFFSET",
	"UNION", "INTERSECT", "EXCEPT", "DISTINCT", "AS",
	"INSERT", "INTO", "VALUES", "UPDATE", "SET", "DELETE",
	"CREATE", "TABLE", "INDEX", "VIEW", "DROP", "ALTER", "TRUNCATE",
	"WITH", "CASE", "WHEN", "THEN", "ELSE", "END",
	"COUNT", "SUM", "AVG", "MIN", "MAX", "COALESCE", "NULLIF",
	"TRUE", "FALSE",
}

var postgresCatalog = BuiltinCatalog{
	ConnectionType: common.Postgres,
	SqlKeywords: append(sqlKeywordsCommon,
		"RETURNING", "ILIKE", "OVERLAPS",
		"SCHEMA", "SEQUENCE", "MATERIALIZED",
		"CONFLICT", "DO", "NOTHING",
		// psql meta-commands — useful as completions for admin stages.
		"\\dt", "\\d", "\\dn", "\\dv", "\\df",
	),
}

var mysqlCatalog = BuiltinCatalog{
	ConnectionType: common.Mysql,
	SqlKeywords: append(sqlKeywordsCommon,
		"SHOW", "DATABASES", "TABLES", "COLUMNS", "STATUS",
		"DESCRIBE", "EXPLAIN", "USE",
		"AUTO_INCREMENT", "UNSIGNED",
		"REPLACE", "DUPLICATE", "KEY",
	),
}

var mongoCatalog = BuiltinCatalog{
	ConnectionType: common.Mongo,
	MongoCollectionOperations: []string{
		// Reads
		"find", "findOne", "aggregate", "distinct",
		"count", "estimatedDocumentCount", "countDocuments",
		// Writes
		"insertOne", "insertMany",
		"updateOne", "updateMany", "replaceOne",
		"deleteOne", "deleteMany",
		"findOneAndUpdate", "findOneAndReplace", "findOneAndDelete",
		"bulkWrite",
		// Meta
		"createIndex", "dropIndex", "getIndexes",
	},
	MongoAggregationOperators: []string{
		// Pipeline stages
		"$match", "$project", "$group", "$sort", "$limit", "$skip",
		"$unwind", "$lookup", "$addFields", "$set", "$unset",
		"$count", "$facet", "$bucket", "$bucketAuto",
		"$sortByCount", "$replaceRoot", "$replaceWith",
		"$sample", "$geoNear", "$graphLookup", "$merge", "$out",
		// Common expression operators (useful inside pipeline stages)
		"$eq", "$ne", "$gt", "$gte", "$lt", "$lte",
		"$in", "$nin", "$and", "$or", "$not", "$nor",
		"$exists", "$type", "$regex",
		"$sum", "$avg", "$min", "$max", "$first", "$last", "$push", "$addToSet",
		"$concat", "$substr", "$toLower", "$toUpper",
		"$size", "$arrayElemAt", "$slice",
	},
}

var redisCatalog = BuiltinCatalog{
	ConnectionType: common.Redis,
	RedisCommands: []string{
		// Strings
		"GET", "SET", "SETEX", "SETNX", "MSET", "MGET",
		"APPEND", "GETSET", "GETDEL",
		"INCR", "INCRBY", "INCRBYFLOAT", "DECR", "DECRBY",
		"STRLEN", "GETRANGE", "SETRANGE",
		// Generic keys
		"DEL", "UNLINK", "EXISTS", "TYPE", "TTL", "PTTL",
		"EXPIRE", "PEXPIRE", "EXPIREAT", "PEXPIREAT", "PERSIST",
		"KEYS", "SCAN", "RENAME", "RENAMENX", "COPY",
		// Hashes
		"HGET", "HSET", "HSETNX", "HMGET", "HMSET", "HGETALL",
		"HKEYS", "HVALS", "HLEN", "HEXISTS", "HDEL",
		"HINCRBY", "HINCRBYFLOAT", "HSCAN",
		// Lists
		"LPUSH", "RPUSH", "LPOP", "RPOP", "LRANGE", "LLEN",
		"LINDEX", "LSET", "LREM", "LTRIM", "LINSERT",
		"BLPOP", "BRPOP", "RPOPLPUSH", "LMOVE", "LMPOP",
		// Sets
		"SADD", "SREM", "SMEMBERS", "SISMEMBER", "SCARD",
		"SPOP", "SRANDMEMBER", "SMOVE", "SSCAN",
		"SINTER", "SUNION", "SDIFF",
		"SINTERSTORE", "SUNIONSTORE", "SDIFFSTORE",
		// Sorted sets
		"ZADD", "ZREM", "ZRANGE", "ZREVRANGE",
		"ZRANGEBYSCORE", "ZREVRANGEBYSCORE", "ZRANGEBYLEX",
		"ZSCORE", "ZCARD", "ZCOUNT", "ZRANK", "ZREVRANK",
		"ZINCRBY", "ZPOPMIN", "ZPOPMAX",
		"ZUNIONSTORE", "ZINTERSTORE", "ZDIFFSTORE", "ZSCAN",
		// Bitmaps and HLL
		"GETBIT", "SETBIT", "BITCOUNT", "BITOP", "BITPOS",
		"PFADD", "PFCOUNT", "PFMERGE",
		// Streams
		"XADD", "XREAD", "XRANGE", "XREVRANGE", "XLEN",
		"XDEL", "XTRIM", "XGROUP", "XACK", "XCLAIM",
		// Server / admin
		"PING", "ECHO", "INFO", "TIME", "DBSIZE",
		"FLUSHDB", "FLUSHALL", "SELECT",
		"CONFIG", "CLIENT", "DEBUG",
		// Pub/Sub
		"PUBLISH", "SUBSCRIBE", "UNSUBSCRIBE", "PSUBSCRIBE", "PUNSUBSCRIBE",
		// Scripting
		"EVAL", "EVALSHA",
	},
}

var jqCatalog = BuiltinCatalog{
	ConnectionType: common.Jq,
	JqOperators: []string{
		// Core syntax tokens
		".", "|", ",", "?", "//",
		// Filters
		"length", "utf8bytelength", "keys", "keys_unsorted",
		"values", "has", "in", "contains", "inside",
		"type", "not", "select", "map", "map_values",
		"to_entries", "from_entries", "with_entries",
		"paths", "leaf_paths", "getpath", "setpath", "delpaths",
		"add", "any", "all", "empty", "error",
		"range", "floor", "ceil", "round", "sqrt",
		"tonumber", "tostring", "ascii_downcase", "ascii_upcase",
		"split", "join", "ltrimstr", "rtrimstr", "startswith", "endswith",
		"test", "match", "capture", "scan", "splits", "sub", "gsub",
		"unique", "unique_by", "group_by", "sort", "sort_by",
		"min", "max", "min_by", "max_by",
		"reverse", "first", "last", "nth",
		"flatten", "reduce", "foreach",
		"recurse", "walk",
		"fromjson", "tojson", "tonumber",
		"isnan", "isinfinite", "infinite", "nan", "null",
	},
}
