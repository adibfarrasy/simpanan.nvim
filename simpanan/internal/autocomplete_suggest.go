package internal

import (
	"simpanan/internal/common"
	"sort"
	"strings"
)

// ComputeSuggestions turns a ContextClassification into a ranked list
// of Suggestions. Filtering and ranking live here — not in the editor —
// so every client (Neovim cmp source, a future LSP server, other
// editor integrations) sees identical results.
//
// Filtering: case-insensitive prefix match on the completion part of
// the classification's Prefix. Qualifiers (`alias.`, `db.`, `db.app.`)
// are stripped before matching.
//
// Ranking: alphabetical within each suggestion kind. Kinds are not
// intermixed — a sql_table_expected context returns all databases
// first, then all tables, both alphabetised.
func ComputeSuggestions(cc ContextClassification) []Suggestion {
	qualifier, completion := splitPrefix(cc.Prefix)

	switch cc.Context {
	case CtxStageStart:
		return suggestStageLabels(completion)
	case CtxSqlKeywordPrefix:
		return suggestSqlKeywords(cc.ConnectionLabel, completion)
	case CtxSqlTableExpected:
		return suggestSqlDatabasesAndTables(cc.ConnectionLabel, completion)
	case CtxSqlColumnExpected:
		return suggestSqlColumns(cc.ConnectionLabel, cc.SqlAliases, qualifier, completion)
	case CtxMongoDatabaseExpected:
		return suggestMongoDatabases(cc.ConnectionLabel, completion)
	case CtxMongoCollectionExpected:
		return suggestMongoCollections(cc.ConnectionLabel, qualifier, completion)
	case CtxMongoOperationExpected:
		return suggestMongoOperations(completion)
	case CtxMongoFieldExpected:
		return suggestMongoFields(cc.ConnectionLabel, completion)
	case CtxRedisCommandPrefix:
		return suggestRedisCommands(completion)
	case CtxJqPlaceholder:
		// Path suggestions are produced by ProbeJqPaths in a later
		// milestone; M6 returns operators only.
		return suggestJqOperators(completion)
	}
	return nil
}

// splitPrefix splits a prefix like `u.col` into ("u", "col") or
// `db.app.` into ("db.app", ""). Everything after the LAST dot is the
// completion filter; everything before it is the qualifier.
func splitPrefix(prefix string) (qualifier, completion string) {
	i := strings.LastIndex(prefix, ".")
	if i < 0 {
		return "", prefix
	}
	return prefix[:i], prefix[i+1:]
}

// filterByPrefix returns a new slice containing only the items whose
// lowercase form starts with the lowercase `prefix`. Empty prefix
// matches everything.
func filterByPrefix(items []string, prefix string) []string {
	if prefix == "" {
		out := make([]string, len(items))
		copy(out, items)
		return out
	}
	p := strings.ToLower(prefix)
	var out []string
	for _, it := range items {
		if strings.HasPrefix(strings.ToLower(it), p) {
			out = append(out, it)
		}
	}
	return out
}

// asSuggestions converts a list of texts to Suggestions of the given
// kind, alphabetised.
func asSuggestions(texts []string, kind SuggestionKind) []Suggestion {
	sorted := append([]string(nil), texts...)
	sort.Strings(sorted)
	out := make([]Suggestion, 0, len(sorted))
	for _, t := range sorted {
		out = append(out, Suggestion{Text: t, Kind: kind})
	}
	return out
}

// ---- stage_start --------------------------------------------------

func suggestStageLabels(prefix string) []Suggestion {
	conns, err := GetConnectionList()
	if err != nil {
		return nil
	}
	labels := make([]string, 0, len(conns))
	for _, c := range conns {
		labels = append(labels, c.Key)
	}
	return asSuggestions(filterByPrefix(labels, prefix), SuggestionConnectionLabel)
}

// ---- sql_keyword_prefix -------------------------------------------

func suggestSqlKeywords(label, prefix string) []Suggestion {
	ct, ok := lookupConnTypeForLabel(label)
	if !ok {
		return nil
	}
	cat := GetBuiltinCatalog(ct)
	return asSuggestions(filterByPrefix(cat.SqlKeywords, prefix), SuggestionSqlKeyword)
}

// ---- sql_table_expected -------------------------------------------

func suggestSqlDatabasesAndTables(label, prefix string) []Suggestion {
	cache, _ := EnsureSchemaCache(label)
	if cache == nil {
		return nil
	}
	var dbNames, tableNames []string
	for _, db := range cache.Databases {
		dbNames = append(dbNames, db.Name)
		for _, t := range db.Tables {
			tableNames = append(tableNames, t.Name)
		}
	}
	out := asSuggestions(filterByPrefix(dbNames, prefix), SuggestionDatabase)
	out = append(out, asSuggestions(filterByPrefix(tableNames, prefix), SuggestionTable)...)
	return out
}

// ---- sql_column_expected ------------------------------------------

func suggestSqlColumns(label string, aliases map[string]string, qualifier, completion string) []Suggestion {
	cache, _ := EnsureSchemaCache(label)
	if cache == nil {
		return nil
	}
	// Build an alias.table → columns map from the cache by flat-scan.
	columnsOf := func(tableName string) []string {
		for _, db := range cache.Databases {
			for _, t := range db.Tables {
				if t.Name == tableName {
					return t.Columns
				}
			}
		}
		return nil
	}

	if qualifier != "" {
		table, ok := aliases[qualifier]
		if !ok {
			// Unknown alias or qualifier that is not a known alias —
			// empty. Do not fall back to union, else user gets noise.
			return nil
		}
		return asSuggestions(filterByPrefix(columnsOf(table), completion), SuggestionColumn)
	}

	// Bare completion: union columns of every aliased table in scope.
	seen := map[string]struct{}{}
	var cols []string
	for _, tableName := range aliases {
		for _, c := range columnsOf(tableName) {
			if _, ok := seen[c]; !ok {
				seen[c] = struct{}{}
				cols = append(cols, c)
			}
		}
	}
	if len(cols) == 0 {
		// If no aliases were extracted, fall back to the union across
		// every cached table — still better than nothing.
		for _, db := range cache.Databases {
			for _, t := range db.Tables {
				for _, c := range t.Columns {
					if _, ok := seen[c]; !ok {
						seen[c] = struct{}{}
						cols = append(cols, c)
					}
				}
			}
		}
	}
	return asSuggestions(filterByPrefix(cols, completion), SuggestionColumn)
}

// ---- mongo_* ------------------------------------------------------

func suggestMongoDatabases(label, prefix string) []Suggestion {
	cache, _ := EnsureSchemaCache(label)
	if cache == nil {
		return nil
	}
	var names []string
	for _, db := range cache.Databases {
		names = append(names, db.Name)
	}
	return asSuggestions(filterByPrefix(names, prefix), SuggestionDatabase)
}

// suggestMongoCollections reads the database name out of the
// qualifier (which will look like "db" or "db.<dbName>"); the leading
// "db." prefix is stripped to leave the real database name.
func suggestMongoCollections(label, qualifier, completion string) []Suggestion {
	cache, _ := EnsureSchemaCache(label)
	if cache == nil {
		return nil
	}
	dbName := strings.TrimPrefix(qualifier, "db.")
	if dbName == "" || dbName == "db" {
		// Qualifier was just "db" (depth 1) — caller really wants
		// databases, but we were routed here by the classifier. Fall
		// back to union across all databases' collections.
		dbName = ""
	}
	var colls []string
	for _, db := range cache.Databases {
		if dbName != "" && db.Name != dbName {
			continue
		}
		for _, c := range db.Collections {
			colls = append(colls, c.Name)
		}
	}
	return asSuggestions(filterByPrefix(colls, completion), SuggestionMongoCollection)
}

func suggestMongoOperations(prefix string) []Suggestion {
	cat := GetBuiltinCatalog(common.Mongo)
	out := asSuggestions(filterByPrefix(cat.MongoCollectionOperations, prefix), SuggestionMongoOperation)
	// Aggregation operators are also useful at the operation-expected
	// position when the user is about to start building a pipeline.
	out = append(out, asSuggestions(filterByPrefix(cat.MongoAggregationOperators, prefix), SuggestionMongoOperator)...)
	return out
}

func suggestMongoFields(label, prefix string) []Suggestion {
	cache, _ := EnsureSchemaCache(label)
	if cache == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var fields []string
	for _, db := range cache.Databases {
		for _, c := range db.Collections {
			for _, f := range c.Fields {
				if _, ok := seen[f]; !ok {
					seen[f] = struct{}{}
					fields = append(fields, f)
				}
			}
		}
	}
	return asSuggestions(filterByPrefix(fields, prefix), SuggestionMongoField)
}

// ---- redis_command_prefix -----------------------------------------

func suggestRedisCommands(prefix string) []Suggestion {
	cat := GetBuiltinCatalog(common.Redis)
	return asSuggestions(filterByPrefix(cat.RedisCommands, prefix), SuggestionRedisCommand)
}

// ---- jq_placeholder (operators only for M6) -----------------------

func suggestJqOperators(prefix string) []Suggestion {
	cat := GetBuiltinCatalog(common.Jq)
	return asSuggestions(filterByPrefix(cat.JqOperators, prefix), SuggestionJqOperator)
}
