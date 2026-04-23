package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"simpanan/internal/common"
	"sort"
	"strings"
	"time"
)

const (
	probeCacheDirName = "simpanan_jq_probe_cache"
	probeCacheTTL     = 30 * time.Second
	mongoArraySampleLimit = 20
)

// runPipelineFn executes a list of stages and returns the final stage's
// JSON output. Swapped in tests to avoid live database I/O.
var runPipelineFn = defaultRunPipeline

func defaultRunPipeline(stages []common.QueryMetadata) ([]byte, error) {
	tmpRes := []byte{}
	for i, q := range stages {
		if i > 0 && len(tmpRes) == 0 {
			return nil, fmt.Errorf("pipeline: stage %d got empty input", i)
		}
		res, err := execute(q, tmpRes)
		if err != nil {
			return nil, err
		}
		tmpRes = res
	}
	return tmpRes, nil
}

// SuggestForBuffer is the top-level entry for completion requests.
// Classifies the cursor position, computes suggestions, and (for
// JqPlaceholder contexts) augments them with jq paths probed from the
// prior stages.
func SuggestForBuffer(bufferText string, cursorPos int) []Suggestion {
	cc := ClassifyContext(bufferText, cursorPos)
	base := ComputeSuggestions(cc)
	if cc.Context != CtxJqPlaceholder {
		return base
	}
	return augmentWithJqPaths(base, bufferText, cursorPos)
}

// augmentWithJqPaths adds jq path suggestions to the base operator
// list. Silently degrades to the base list on any failure.
func augmentWithJqPaths(base []Suggestion, bufferText string, cursorPos int) []Suggestion {
	priors, err := extractPriorStages(bufferText, cursorPos)
	if err != nil || len(priors) == 0 {
		return base
	}
	timeout := AutocompleteConfig().JqPathProbeTimeout

	payload, ok := probeWithCache(priors, timeout)
	if !ok {
		return base
	}

	paths, err := extractJqPaths(payload)
	if err != nil {
		return base
	}
	placeholderPrefix := jqPrefixInPlaceholder(bufferText[:cursorPos])
	filtered := filterByPrefix(paths, placeholderPrefix)
	return append(base, asSuggestions(filtered, SuggestionJqPath)...)
}

// probeWithCache returns the prior-pipeline output, using the on-disk
// cache when a recent entry exists and falling back to executing the
// pipeline within the given timeout. ok=false means the probe was
// unsuccessful (timed out, errored, nothing to return) and the caller
// should degrade to operators-only.
func probeWithCache(stages []common.QueryMetadata, timeout time.Duration) ([]byte, bool) {
	hash := pipelineHash(stages)

	if cached, ts, err := loadProbeResult(hash); err == nil && cached != nil {
		if time.Since(ts) < probeCacheTTL {
			return cached, true
		}
	}

	type result struct {
		payload []byte
		err     error
	}
	done := make(chan result, 1)
	go func() {
		payload, err := runPipelineFn(stages)
		done <- result{payload, err}
	}()

	select {
	case r := <-done:
		if r.err != nil || len(r.payload) == 0 {
			return nil, false
		}
		_ = saveProbeResult(hash, r.payload)
		return r.payload, true
	case <-time.After(timeout):
		// The goroutine keeps running; if it eventually succeeds it
		// will populate the cache for the next request.
		return nil, false
	}
}

// extractPriorStages parses the buffer into stages and returns
// QueryMetadata for every stage strictly before the cursor's stage.
func extractPriorStages(bufferText string, cursorPos int) ([]common.QueryMetadata, error) {
	if cursorPos > len(bufferText) {
		cursorPos = len(bufferText)
	}
	stages := parseBufferStages(bufferText)
	if len(stages) == 0 {
		return nil, nil
	}
	// Current stage = last stage whose HeaderStart <= cursorPos.
	currentIdx := -1
	for i, s := range stages {
		if s.HeaderStart <= cursorPos {
			currentIdx = i
		}
	}
	if currentIdx <= 0 {
		return nil, nil
	}
	prior := stages[:currentIdx]

	conns, err := GetConnectionList()
	if err != nil {
		return nil, err
	}
	connMap := map[string]string{}
	for _, c := range conns {
		connMap[c.Key] = string(c.URI)
	}
	connMap["jq"] = "jq://"

	var out []common.QueryMetadata
	for _, s := range prior {
		uri, ok := connMap[s.Label]
		if !ok {
			return nil, fmt.Errorf("extractPriorStages: unknown label %q", s.Label)
		}
		ct, err := common.URI(uri).ConnType()
		if err != nil {
			return nil, err
		}
		out = append(out, common.QueryMetadata{
			Conn:      uri,
			ConnType:  *ct,
			QueryLine: strings.TrimSpace(s.Query),
		})
	}
	return out, nil
}

// bufferStage is a single stage parsed out of a raw buffer.
type bufferStage struct {
	Label       string
	Query       string
	HeaderStart int // byte offset of the header line's first char
}

// parseBufferStages splits a .simp buffer into its stages. Lines
// starting with `//` are skipped. Continuation lines (no header) are
// concatenated onto the current stage's query.
func parseBufferStages(bufferText string) []bufferStage {
	var out []bufferStage
	lines := strings.Split(bufferText, "\n")
	offset := 0
	for _, line := range lines {
		trimmedLeft := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmedLeft, "//") {
			offset += len(line) + 1
			continue
		}
		if m := stageHeaderRe.FindStringSubmatch(line); m != nil {
			headerEnd := strings.Index(line, ">") + 1
			query := strings.TrimSpace(line[headerEnd:])
			out = append(out, bufferStage{
				Label:       m[1],
				Query:       query,
				HeaderStart: offset,
			})
		} else if len(out) > 0 {
			cont := strings.TrimSpace(line)
			if cont != "" {
				cur := &out[len(out)-1]
				if cur.Query == "" {
					cur.Query = cont
				} else {
					cur.Query = cur.Query + " " + cont
				}
			}
		}
		offset += len(line) + 1
	}
	return out
}

// jqPrefixInPlaceholder returns the text between the most recent `{{`
// and the cursor, or "" if the cursor is not inside one.
func jqPrefixInPlaceholder(before string) string {
	open := strings.LastIndex(before, "{{")
	if open < 0 {
		return ""
	}
	close := strings.LastIndex(before, "}}")
	if close > open {
		return ""
	}
	return before[open+2:]
}

// extractJqPaths walks a JSON tree and collects unique jq-style paths.
// Array indices are generalised to `[]` so `.users[0].id` and
// `.users[1].id` collapse to `.users[].id`. Paths are returned sorted.
func extractJqPaths(payload []byte) ([]string, error) {
	var root interface{}
	if err := json.Unmarshal(payload, &root); err != nil {
		return nil, err
	}
	set := map[string]struct{}{}
	walkJqPaths("", root, set)
	out := make([]string, 0, len(set))
	for p := range set {
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

func walkJqPaths(current string, v interface{}, out map[string]struct{}) {
	if current != "" {
		out[current] = struct{}{}
	}
	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			next := "." + k
			if current != "" {
				next = current + "." + k
			}
			walkJqPaths(next, child, out)
		}
	case []interface{}:
		if len(val) == 0 {
			return
		}
		next := "[]"
		if current != "" {
			next = current + "[]"
		} else {
			next = ".[]"
		}
		// Sample a few elements to discover keys that may differ
		// between items (Mongo-ish heterogeneous collections).
		for i := 0; i < len(val) && i < mongoArraySampleLimit; i++ {
			walkJqPaths(next, val[i], out)
		}
	}
}

// pipelineHash derives a stable SHA-256 over the ordered list of
// stages so the probe cache can be keyed by "the exact prior pipeline".
func pipelineHash(stages []common.QueryMetadata) string {
	h := sha256.New()
	for _, s := range stages {
		fmt.Fprintf(h, "%s\x00%s\x00%s\n", s.ConnType, s.Conn, s.QueryLine)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// probeCacheDir resolves the on-disk cache directory. Created lazily
// by saveProbeResult.
func probeCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local/share/nvim", probeCacheDirName), nil
}

func probeCachePath(hash string) (string, error) {
	dir, err := probeCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, hash+".json"), nil
}

// loadProbeResult returns the cached payload plus its mtime. Missing
// file yields (nil, zero, nil).
func loadProbeResult(hash string) ([]byte, time.Time, error) {
	path, err := probeCachePath(hash)
	if err != nil {
		return nil, time.Time{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, time.Time{}, nil
		}
		return nil, time.Time{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	return data, info.ModTime(), nil
}

// saveProbeResult writes atomically (temp file + rename).
func saveProbeResult(hash string, payload []byte) error {
	path, err := probeCachePath(hash)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".probe-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
