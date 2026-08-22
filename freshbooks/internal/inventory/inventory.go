package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Family classifies which FreshBooks API family a request belongs to.
const (
	FamilyAccounting = "accounting"
	FamilyBusiness   = "business"
	FamilyAuth       = "auth"
	FamilyEvents     = "events"
	FamilyUploads    = "uploads"
	FamilyPayments   = "payments"
	FamilyLedger     = "ledger"
	FamilyInternal   = "internal"
	FamilyUnknown    = "unknown"
)

// internalHost and internalPrefix identify the my.freshbooks.com internal
// endpoints (see design spec section 5.2, rule 4) that get rewritten to
// their public api.freshbooks.com equivalent and marked family "internal".
const (
	internalHost   = "my.freshbooks.com"
	internalPrefix = "/service/api"
)

// Entry is one normalized inventory entry. Field order matches the design
// spec exactly and is preserved on JSON marshal so testdata/inventory.json
// stays byte-stable across regenerations.
type Entry struct {
	Key          string       `json:"key"`
	Folder       string       `json:"folder"`
	Path         []string     `json:"path"`
	Name         string       `json:"name"`
	Method       string       `json:"method"`
	PathTemplate string       `json:"pathTemplate"`
	Host         string       `json:"host"`
	Query        []QueryEntry `json:"query"`
	Body         *string      `json:"body"`
	Responses    []RespEntry  `json:"responses"`
	Family       string       `json:"family"`
	Duplicates   int          `json:"duplicates"`
}

// QueryEntry is one normalized query parameter.
type QueryEntry struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// RespEntry is one normalized example response.
type RespEntry struct {
	Name   string `json:"name"`
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// Load reads and parses a Postman v2.1 collection file.
func Load(path string) (*Collection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("inventory: reading %s: %w", path, err)
	}
	var c Collection
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("inventory: parsing %s: %w", path, err)
	}
	return &c, nil
}

// Normalize walks a parsed collection and returns its normalized,
// deduplicated entries sorted by Key. Two source requests with the same key
// (folder/subfolders/name, trimmed) and identical method+pathTemplate
// collapse into one entry with Duplicates incremented. Two source requests
// with the same key but a different method or pathTemplate are a genuine
// name collision (the FreshBooks Postman collection reuses "Single Tax" for
// both its GET and DELETE requests in two folders); Normalize disambiguates
// by suffixing later collisions with " (METHOD)" rather than silently
// dropping data, and only fails if that suffix itself collides.
func Normalize(c *Collection) ([]Entry, error) {
	var raw []Entry
	if err := walk(c.Item, nil, &raw); err != nil {
		return nil, err
	}
	return dedupe(raw)
}

func walk(items []Item, trail []string, out *[]Entry) error {
	for _, it := range items {
		if len(it.Item) > 0 {
			nextTrail := make([]string, len(trail)+1)
			copy(nextTrail, trail)
			nextTrail[len(trail)] = it.Name
			if err := walk(it.Item, nextTrail, out); err != nil {
				return err
			}
			continue
		}
		if it.Request == nil {
			continue
		}
		entry, err := buildEntry(trail, it)
		if err != nil {
			return err
		}
		*out = append(*out, entry)
	}
	return nil
}

func buildEntry(trail []string, it Item) (Entry, error) {
	if len(trail) == 0 {
		return Entry{}, fmt.Errorf("inventory: request %q has no enclosing top-level folder", it.Name)
	}

	folder := strings.TrimSpace(trail[0])
	path := make([]string, 0, len(trail)-1)
	for _, seg := range trail[1:] {
		path = append(path, strings.TrimSpace(seg))
	}
	name := strings.TrimSpace(it.Name)

	keySegs := append([]string{folder}, path...)
	keySegs = append(keySegs, name)
	key := strings.Join(keySegs, "/")

	method := strings.ToUpper(strings.TrimSpace(it.Request.Method))

	host, pathTemplate, query, family, err := normalizeURL(it.Request.URL)
	if err != nil {
		return Entry{}, fmt.Errorf("inventory: entry %q: %w", key, err)
	}

	return Entry{
		Key:          key,
		Folder:       folder,
		Path:         path,
		Name:         name,
		Method:       method,
		PathTemplate: pathTemplate,
		Host:         host,
		Query:        query,
		Body:         extractBody(it.Request.Body),
		Responses:    extractResponses(it.Response),
		Family:       family,
		Duplicates:   1,
	}, nil
}

func extractBody(b *Body) *string {
	if b == nil || b.Mode != "raw" {
		return nil
	}
	raw := b.Raw
	return &raw
}

func extractResponses(rs []Response) []RespEntry {
	out := make([]RespEntry, 0, len(rs))
	for _, r := range rs {
		out = append(out, RespEntry{
			Name:   strings.TrimSpace(r.Name),
			Status: r.Code,
			Body:   r.Body,
		})
	}
	return out
}

// normalizeURL strips whitespace from the raw URL, applies the
// my.freshbooks.com internal-host rewrite, replaces Postman variables and
// hard-coded IDs in the path with their template placeholders, and
// classifies the request's API family.
func normalizeURL(u URL) (host, pathTemplate string, query []QueryEntry, family string, err error) {
	raw := stripWhitespace(u.Raw)
	parsed, perr := url.Parse(raw)
	if perr != nil {
		return "", "", nil, "", fmt.Errorf("parsing url %q: %w", u.Raw, perr)
	}

	host = parsed.Host
	path := parsed.Path
	internal := false
	if host == internalHost && strings.HasPrefix(path, internalPrefix) {
		host = "api.freshbooks.com"
		path = strings.TrimPrefix(path, internalPrefix)
		if path == "" {
			path = "/"
		}
		internal = true
	}

	pathTemplate = normalizePathSegments(path)

	if internal {
		family = FamilyInternal
	} else {
		family = classifyFamily(pathTemplate)
	}

	if u.FromObject {
		query = make([]QueryEntry, 0, len(u.Query))
		for _, q := range u.Query {
			query = append(query, QueryEntry{
				Name:        strings.TrimSpace(q.Key),
				Value:       substituteVars(q.Value),
				Description: q.Description,
			})
		}
	} else {
		query = normalizeQueryString(parsed.RawQuery)
	}

	return host, pathTemplate, query, family, nil
}

func stripWhitespace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\v' || r == '\f' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func normalizeQueryString(rawQuery string) []QueryEntry {
	out := make([]QueryEntry, 0)
	if rawQuery == "" {
		return out
	}
	for _, part := range strings.Split(rawQuery, "&") {
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		name, _ := url.QueryUnescape(kv[0])
		value := ""
		if len(kv) > 1 {
			value, _ = url.QueryUnescape(kv[1])
		}
		out = append(out, QueryEntry{
			Name:  name,
			Value: substituteVars(value),
		})
	}
	return out
}

var (
	postmanVarWhole = regexp.MustCompile(`^\{\{([^{}]+)\}\}$`)
	postmanVarAny   = regexp.MustCompile(`\{\{([^{}]+)\}\}`)
	numericSegment  = regexp.MustCompile(`^[0-9]+$`)
	wordSplitter    = regexp.MustCompile(`[^a-z0-9]+`)
)

// acronyms gives fixed casing for known acronym words when they appear
// after the first word of a normalized variable name.
var acronyms = map[string]string{
	"id":   "Id",
	"uuid": "Uuid",
}

// acronymSuffixes are acronym words that can appear glued to a preceding
// word with no separator (e.g. "accountid"); checked longest-first so
// "uuid" is not mistaken for a trailing "id".
var acronymSuffixes = []string{"uuid", "id"}

func normalizePathSegments(path string) string {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if seg == "" {
			continue
		}
		if m := postmanVarWhole.FindStringSubmatch(seg); m != nil {
			segments[i] = "{" + normalizeVarName(m[1]) + "}"
			continue
		}
		if i > 0 && segments[i-1] == "account" {
			segments[i] = "{accountId}"
			continue
		}
		if i > 0 && segments[i-1] == "business" && numericSegment.MatchString(seg) {
			segments[i] = "{businessId}"
			continue
		}
		if numericSegment.MatchString(seg) {
			segments[i] = "{id}"
			continue
		}
	}
	return strings.Join(segments, "/")
}

func classifyFamily(path string) string {
	switch {
	case strings.HasPrefix(path, "/accounting/account/"):
		return FamilyAccounting
	case strings.HasPrefix(path, "/accounting/businesses/"):
		return FamilyLedger
	case strings.HasPrefix(path, "/projects/business/"),
		strings.HasPrefix(path, "/timetracking/business/"),
		strings.HasPrefix(path, "/comments/business/"),
		strings.HasPrefix(path, "/auth/api/v1/businesses/"):
		return FamilyBusiness
	case strings.HasPrefix(path, "/auth/"):
		return FamilyAuth
	case strings.HasPrefix(path, "/events/"):
		return FamilyEvents
	case strings.HasPrefix(path, "/uploads/"):
		return FamilyUploads
	case strings.HasPrefix(path, "/payments/"):
		return FamilyPayments
	default:
		return FamilyUnknown
	}
}

// substituteVars replaces every {{name}} occurrence in s with its
// normalized {name} form, leaving the rest of s untouched.
func substituteVars(s string) string {
	return postmanVarAny.ReplaceAllStringFunc(s, func(m string) string {
		name := m[2 : len(m)-2]
		return "{" + normalizeVarName(name) + "}"
	})
}

// normalizeVarName lower-camel-cases a Postman variable name: lowercase
// everything, split on non-alphanumeric separators, peel a glued-on
// acronym suffix ("accountid" -> "account"+"id"), then rejoin with the
// first word lowercase and every later word TitleCased (or its fixed
// acronym form).
func normalizeVarName(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	var words []string
	for _, w := range wordSplitter.Split(lower, -1) {
		if w == "" {
			continue
		}
		words = append(words, peelAcronymSuffix(w)...)
	}
	if len(words) == 0 {
		return lower
	}

	var b strings.Builder
	for i, w := range words {
		if i == 0 {
			b.WriteString(w)
			continue
		}
		if mapped, ok := acronyms[w]; ok {
			b.WriteString(mapped)
		} else {
			b.WriteString(titleCaseWord(w))
		}
	}
	return b.String()
}

func peelAcronymSuffix(w string) []string {
	if _, isWholeAcronym := acronyms[w]; isWholeAcronym {
		return []string{w}
	}
	for _, suf := range acronymSuffixes {
		if len(w) > len(suf) && strings.HasSuffix(w, suf) {
			return []string{w[:len(w)-len(suf)], suf}
		}
	}
	return []string{w}
}

func titleCaseWord(w string) string {
	if w == "" {
		return w
	}
	r := []rune(w)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// dedupe collapses exact duplicates (same key, method, and pathTemplate)
// into one Entry with Duplicates incremented, disambiguates genuine name
// collisions (same key, different method or pathTemplate) by suffixing the
// later entry's key with " (METHOD)", and sorts the result by Key so
// re-emitting the same collection is byte-stable.
func dedupe(entries []Entry) ([]Entry, error) {
	result := make([]Entry, 0, len(entries))
	sigIndex := make(map[string]int, len(entries))
	baseKeyCount := make(map[string]int, len(entries))

	for _, e := range entries {
		baseKey := e.Key
		sigKey := baseKey + "\x00" + e.Method + "\x00" + e.PathTemplate
		if idx, ok := sigIndex[sigKey]; ok {
			result[idx].Duplicates++
			continue
		}

		finalKey := baseKey
		if baseKeyCount[baseKey] > 0 {
			finalKey = fmt.Sprintf("%s (%s)", baseKey, e.Method)
		}
		for _, existing := range result {
			if existing.Key == finalKey {
				return nil, fmt.Errorf(
					"inventory: %q and an earlier entry both resolve to key %q with conflicting method/path; disambiguation collided",
					e.Name, finalKey,
				)
			}
		}

		e.Key = finalKey
		result = append(result, e)
		sigIndex[sigKey] = len(result) - 1
		baseKeyCount[baseKey]++
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result, nil
}

// WriteJSON marshals entries as sorted, 2-space-indented JSON with a
// trailing newline, matching the format testdata/inventory.json is
// committed in so regeneration is byte-stable.
func WriteJSON(path string, entries []Entry) error {
	sorted := make([]Entry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })

	data, err := json.MarshalIndent(sorted, "", "  ")
	if err != nil {
		return fmt.Errorf("inventory: encoding: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("inventory: writing %s: %w", path, err)
	}
	return nil
}

// ReadJSON reads a previously written inventory.json.
func ReadJSON(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("inventory: reading %s: %w", path, err)
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("inventory: parsing %s: %w", path, err)
	}
	return entries, nil
}
