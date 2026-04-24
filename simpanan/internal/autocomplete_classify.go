package internal

import (
	"regexp"
	"simpanan/internal/common"
	"strings"
)

// CompletionContext is the classified position of the cursor within a
// .simp buffer. Each variant maps to a specific suggestion strategy in
// ComputeSuggestions.
type CompletionContext string

const (
	CtxStageStart              CompletionContext = "stage_start"
	CtxSqlKeywordPrefix        CompletionContext = "sql_keyword_prefix"
	CtxSqlTableExpected        CompletionContext = "sql_table_expected"
	CtxSqlColumnExpected       CompletionContext = "sql_column_expected"
	CtxMongoDatabaseExpected   CompletionContext = "mongo_database_expected"
	CtxMongoCollectionExpected CompletionContext = "mongo_collection_expected"
	CtxMongoOperationExpected  CompletionContext = "mongo_operation_expected"
	CtxMongoFieldExpected      CompletionContext = "mongo_field_expected"
	CtxRedisCommandPrefix      CompletionContext = "redis_command_prefix"
	CtxJqPlaceholder           CompletionContext = "jq_placeholder"
	CtxUnknown                 CompletionContext = "unknown"
)

// ContextClassification is the output of ClassifyContext. Fields beyond
// Context are best-effort: SqlAliases is empty for non-SQL stages,
// ConnectionLabel is empty when the cursor is not inside any stage.
type ContextClassification struct {
	Context         CompletionContext
	Prefix          string
	StageIndex      int
	ConnectionLabel string
	SqlAliases      map[string]string
}

// stageHeaderRe matches the start of a stage line: literal '|', a
// label (no whitespace, no '>', no '|'), optional whitespace, then '>'.
// The leading '|' makes stage starts unambiguous and gives the
// connection-label autocomplete a deliberate trigger character.
var stageHeaderRe = regexp.MustCompile(`^\|([^\s>|]+)\s*>`)

// sqlAliasRe extracts top-level FROM/JOIN aliases of the form
// `<table> [AS] <alias>`. Best-effort; CTE/subquery aliases are out of
// scope for this milestone.
var sqlAliasRe = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+([a-zA-Z_][a-zA-Z0-9_]*)\s+(?:AS\s+)?([a-zA-Z_][a-zA-Z0-9_]*)\b`)

// wordPrefixRe matches the trailing identifier-or-dotted-path before
// the cursor.
var wordPrefixRe = regexp.MustCompile(`[a-zA-Z_$][a-zA-Z0-9_.$\\]*$`)

// ClassifyContext determines what the user is trying to type at
// `cursorPos` in `bufferText`. The result drives ComputeSuggestions.
// The classifier is deliberately a black box per the spec — only the
// shape of ContextClassification is observable.
func ClassifyContext(bufferText string, cursorPos int) ContextClassification {
	if cursorPos < 0 {
		cursorPos = 0
	}
	if cursorPos > len(bufferText) {
		cursorPos = len(bufferText)
	}
	before := bufferText[:cursorPos]

	stageIdx, headerEnd, label := findCurrentStage(before)
	prefix := wordPrefixAt(before)

	// In-progress new stage header: the cursor's current line starts
	// (after optional whitespace) with '|' but has no '>' yet. The
	// user is typing a brand-new stage header, so we must NOT classify
	// the cursor as belonging to whatever previous stage's body
	// findCurrentStage just located. Suggest connection labels.
	{
		lineStart := lastIndexOrZero(before, "\n")
		currentLine := before[lineStart:]
		trimmed := strings.TrimLeft(currentLine, " \t")
		if strings.HasPrefix(trimmed, "|") && !strings.Contains(trimmed, ">") {
			labelPrefix := strings.TrimPrefix(trimmed, "|")
			return ContextClassification{
				Context:    CtxStageStart,
				Prefix:     labelPrefix,
				StageIndex: stageIdx + 1,
			}
		}
	}

	// No stage header before cursor: user is typing the label of the
	// first stage (or is in pre-stage whitespace).
	if headerEnd < 0 {
		return ContextClassification{
			Context:    CtxStageStart,
			Prefix:     prefix,
			StageIndex: 0,
		}
	}

	// Cursor is on the same line as the header but BEFORE the '>': they
	// are still typing the label. STRICTLY less than: cursor exactly at
	// headerEnd (just past the '>') means the label is committed and we
	// should be classifying the body, not still suggesting labels.
	lineStart := lastIndexOrZero(before, "\n")
	if cursorPos < lineStart+headerEndOffset(before, lineStart) {
		return ContextClassification{
			Context:    CtxStageStart,
			Prefix:     prefix,
			StageIndex: stageIdx,
		}
	}

	stageContent := bufferText[headerEnd:cursorPos]
	contextual := classifyByConnLabel(label, stageContent, prefix)
	contextual.StageIndex = stageIdx
	contextual.ConnectionLabel = label
	if contextual.Prefix == "" {
		contextual.Prefix = prefix
	}
	return contextual
}

// classifyByConnLabel dispatches per the connection's type. Looks up
// the type in the registry; on failure returns Unknown.
func classifyByConnLabel(label, stageContent, prefix string) ContextClassification {
	// jq placeholders are connection-type-agnostic: a {{ in any
	// non-first stage opens a jq context.
	if inJqPlaceholder(stageContent) {
		return ContextClassification{Context: CtxJqPlaceholder, Prefix: prefix}
	}

	// The reserved 'jq' label always behaves as a jq expression.
	if label == "jq" {
		return ContextClassification{Context: CtxJqPlaceholder, Prefix: prefix}
	}

	ct, ok := lookupConnTypeForLabel(label)
	if !ok {
		return ContextClassification{Context: CtxUnknown, Prefix: prefix}
	}

	switch ct {
	case common.Postgres, common.Mysql:
		out := classifySql(stageContent, prefix)
		return out
	case common.Mongo:
		return classifyMongo(stageContent, prefix)
	case common.Redis:
		return ContextClassification{Context: CtxRedisCommandPrefix, Prefix: prefix}
	case common.Jq:
		return ContextClassification{Context: CtxJqPlaceholder, Prefix: prefix}
	}
	return ContextClassification{Context: CtxUnknown, Prefix: prefix}
}

// lookupConnTypeForLabel resolves a connection label to its type via
// the on-disk registry. The reserved "jq" label is recognised without
// a registry lookup.
func lookupConnTypeForLabel(label string) (common.ConnType, bool) {
	if label == "jq" {
		return common.Jq, true
	}
	conns, err := GetConnectionList()
	if err != nil {
		return "", false
	}
	for _, c := range conns {
		if c.Key == label {
			ct, err := common.URI(c.URI).ConnType()
			if err != nil {
				return "", false
			}
			return *ct, true
		}
	}
	return "", false
}

// classifySql picks among the SQL-flavoured contexts based on the most
// recent significant keyword before the cursor. Aliases extracted from
// the same stage's FROM/JOIN clauses are attached to the result.
func classifySql(stageContent, prefix string) ContextClassification {
	aliases := extractSqlAliases(stageContent)

	// `alias.` qualifier takes precedence over keyword scan.
	if dot := strings.LastIndex(prefix, "."); dot > 0 {
		alias := prefix[:dot]
		if _, ok := aliases[alias]; ok {
			return ContextClassification{
				Context:    CtxSqlColumnExpected,
				Prefix:     prefix,
				SqlAliases: aliases,
			}
		}
	}

	switch lastSignificantSqlKeyword(stageContent) {
	case "":
		return ContextClassification{
			Context:    CtxSqlKeywordPrefix,
			Prefix:     prefix,
			SqlAliases: aliases,
		}
	case "FROM", "JOIN", "INTO", "UPDATE":
		return ContextClassification{
			Context:    CtxSqlTableExpected,
			Prefix:     prefix,
			SqlAliases: aliases,
		}
	case "WHERE", "ON", "AND", "OR", "BY":
		// "BY" covers ORDER BY / GROUP BY column positions.
		return ContextClassification{
			Context:    CtxSqlColumnExpected,
			Prefix:     prefix,
			SqlAliases: aliases,
		}
	}
	return ContextClassification{
		Context:    CtxSqlKeywordPrefix,
		Prefix:     prefix,
		SqlAliases: aliases,
	}
}

// classifyMongo recognises the `db.<db>.<coll>.<op>` ladder and
// $-operator field positions.
func classifyMongo(stageContent, prefix string) ContextClassification {
	trimmed := strings.TrimSpace(stageContent)

	// The dotted prefix tells us how many levels into db.X.Y. we are.
	// Match against the END of the trimmed content.
	switch {
	case dottedPathDepth(trimmed) == 1 && strings.HasSuffix(trimmed, "db."):
		return ContextClassification{Context: CtxMongoDatabaseExpected, Prefix: prefix}
	case dottedPathDepth(trimmed) == 2 && hasDbCollectionPrefix(trimmed):
		return ContextClassification{Context: CtxMongoCollectionExpected, Prefix: prefix}
	case dottedPathDepth(trimmed) >= 3 && hasDbCollectionPrefix(trimmed):
		return ContextClassification{Context: CtxMongoOperationExpected, Prefix: prefix}
	}

	// Inside a $match / $group / $project / etc. object literal the
	// cursor is in field-name position.
	if insideMongoOperatorObject(stageContent) {
		return ContextClassification{Context: CtxMongoFieldExpected, Prefix: prefix}
	}

	return ContextClassification{Context: CtxUnknown, Prefix: prefix}
}

// findCurrentStage scans the prefix of the buffer up to the cursor and
// returns the index of the current stage, the byte offset just after
// the most recent stage header's `>`, and the header's label. Returns
// (-1, -1, "") if no stage header has been seen yet.
func findCurrentStage(before string) (int, int, string) {
	idx := -1
	headerEnd := -1
	label := ""
	for lineStart := 0; lineStart <= len(before); {
		nextNL := strings.IndexByte(before[lineStart:], '\n')
		var line string
		if nextNL < 0 {
			line = before[lineStart:]
		} else {
			line = before[lineStart : lineStart+nextNL]
		}
		if m := stageHeaderRe.FindStringSubmatchIndex(line); m != nil {
			idx++
			label = line[m[2]:m[3]]
			headerEnd = lineStart + m[1]
		}
		if nextNL < 0 {
			break
		}
		lineStart += nextNL + 1
	}
	return idx, headerEnd, label
}

// headerEndOffset returns the offset within the line beginning at
// `lineStart` of the position just after the stage header's `>`, or
// 0 if the line is not a header.
func headerEndOffset(before string, lineStart int) int {
	line := before[lineStart:]
	if m := stageHeaderRe.FindStringIndex(line); m != nil {
		return m[1]
	}
	return 0
}

func lastIndexOrZero(s, sep string) int {
	i := strings.LastIndex(s, sep)
	if i < 0 {
		return 0
	}
	return i + 1
}

// wordPrefixAt returns the trailing identifier-like token at the end of
// `before`. Empty if the cursor is on whitespace or punctuation.
func wordPrefixAt(before string) string {
	return wordPrefixRe.FindString(before)
}

// inJqPlaceholder reports whether the cursor is inside an open `{{`
// without a closing `}}` between it and the cursor.
func inJqPlaceholder(stageContent string) bool {
	open := strings.LastIndex(stageContent, "{{")
	if open < 0 {
		return false
	}
	close := strings.LastIndex(stageContent, "}}")
	return close < open
}

// significantSqlKeywords is the set of keywords whose appearance just
// before the cursor pins down the completion context. Order is by
// length-desc so longer phrases match first.
var significantSqlKeywords = []string{
	"DELETE FROM", "INSERT INTO",
	"FROM", "JOIN", "WHERE", "INTO", "UPDATE", "ON", "AND", "OR", "BY",
}

// lastSignificantSqlKeyword returns the most recent context-pinning
// keyword in the stage content, or "" if none. Comparison is
// case-insensitive; multi-word phrases collapse to their first word
// for the dispatcher.
func lastSignificantSqlKeyword(stageContent string) string {
	upper := strings.ToUpper(stageContent)
	bestPos := -1
	bestKw := ""
	for _, kw := range significantSqlKeywords {
		i := strings.LastIndex(upper, kw)
		if i < 0 {
			continue
		}
		// Must be a whole-word boundary on both sides.
		if !boundaryAt(upper, i, kw) {
			continue
		}
		if i > bestPos {
			bestPos = i
			bestKw = kw
		}
	}
	switch bestKw {
	case "DELETE FROM":
		return "FROM"
	case "INSERT INTO":
		return "INTO"
	}
	return bestKw
}

func boundaryAt(s string, pos int, word string) bool {
	if pos > 0 {
		c := s[pos-1]
		if c != ' ' && c != '\t' && c != '\n' && c != '(' && c != ',' {
			return false
		}
	}
	end := pos + len(word)
	if end < len(s) {
		c := s[end]
		if c != ' ' && c != '\t' && c != '\n' && c != ';' && c != '(' && c != ',' {
			return false
		}
	}
	return true
}

// extractSqlAliases pulls top-level FROM/JOIN <table> [AS] <alias>
// pairs from the stage content.
func extractSqlAliases(stageContent string) map[string]string {
	out := map[string]string{}
	for _, m := range sqlAliasRe.FindAllStringSubmatch(stageContent, -1) {
		table, alias := m[1], m[2]
		// Reject pairs whose 'alias' is itself a SQL keyword
		// (e.g. `FROM users WHERE`) — only the first capture is the
		// table, the second would be the keyword in that case.
		if isSqlKeyword(strings.ToUpper(alias)) {
			continue
		}
		out[alias] = table
	}
	return out
}

func isSqlKeyword(upper string) bool {
	for _, kw := range significantSqlKeywords {
		if upper == kw || strings.HasPrefix(kw, upper+" ") {
			return true
		}
	}
	switch upper {
	case "AS", "ORDER", "GROUP", "HAVING", "LIMIT", "OFFSET",
		"INNER", "OUTER", "LEFT", "RIGHT", "FULL", "CROSS", "USING",
		"SELECT", "VALUES", "SET":
		return true
	}
	return false
}

// dottedPathDepth counts dot-separated identifiers at the end of s.
// "db." → 1, "db.app." → 2, "db.app.users." → 3, "db.app.users.find" → 3.
func dottedPathDepth(s string) int {
	// Take the trailing identifier-and-dot run.
	tail := ""
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '.' {
			tail = string(c) + tail
			continue
		}
		break
	}
	if tail == "" {
		return 0
	}
	return strings.Count(tail, ".")
}

// hasDbCollectionPrefix reports whether the trailing dotted path
// begins with `db.`.
func hasDbCollectionPrefix(s string) bool {
	tail := ""
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '.' {
			tail = string(c) + tail
			continue
		}
		break
	}
	return strings.HasPrefix(tail, "db.")
}

// mongoFieldOpRe matches an open object literal directly preceded by a
// $-operator name (e.g. `$match: {`, `$project:{`, `$group:  {`). The
// classifier uses this to detect that the cursor is inside an object
// where field-name completion is appropriate.
var mongoFieldOpRe = regexp.MustCompile(`\$(?:match|project|group|sort|lookup|addFields|set|unset|bucket|bucketAuto|sortByCount|replaceRoot|replaceWith|merge|out|geoNear|graphLookup)\s*:\s*\{`)

// insideMongoOperatorObject returns true when the cursor sits inside
// the most recent `$op: {` whose closing `}` has not yet appeared.
func insideMongoOperatorObject(stageContent string) bool {
	matches := mongoFieldOpRe.FindAllStringIndex(stageContent, -1)
	if len(matches) == 0 {
		return false
	}
	lastOpen := matches[len(matches)-1][1] - 1 // position of the '{'
	// Count braces from lastOpen to end of stageContent.
	depth := 0
	for i := lastOpen; i < len(stageContent); i++ {
		switch stageContent[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return false
			}
		}
	}
	return depth > 0
}
