package freshbooks

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// AccountID is the string account identifier used by the accounting API
// family, rooted at /accounting/account/{account_id}/.... It is never
// interchangeable with BusinessID; both are returned by IdentityService.Me.
type AccountID string

// BusinessID is the integer business identifier used by the business-scoped
// API family (projects, time tracking, comments, and the auth businesses
// endpoints). It is never interchangeable with AccountID.
type BusinessID int64

// String renders the business ID as the decimal digits used in URL paths.
func (b BusinessID) String() string { return strconv.FormatInt(int64(b), 10) }

// BusinessUUID is the UUID form of a business identifier, used only by the
// ledger-accounts endpoints under /accounting/businesses/{business_uuid}/.
type BusinessUUID string

// Family classifies which FreshBooks API family a request belongs to. The
// families differ in URL root, identifier type, response envelope, error
// shape, and query encoding, so the transport needs to know which one it is
// talking to.
//
// These three are the envelope shapes, not a one-to-one map of the inventory
// tool's classifier: that tool also names "events", "uploads", "payments",
// and "ledger" families, which the transport folds into one of the three
// below by the envelope they actually return. Which envelope each of those
// uses is INFERRED from the Postman examples; the Phase 2 batch that
// implements one confirms it live.
type Family string

// The API families this library distinguishes.
const (
	// FamilyAccounting covers /accounting/account/{account_id}/... and the
	// ledger-account endpoints: enveloped responses, errno-bearing errors.
	FamilyAccounting Family = "accounting"
	// FamilyBusiness covers the business-scoped roots (projects,
	// timetracking, comments): flat bodies with a "meta" pagination object.
	FamilyBusiness Family = "business"
	// FamilyAuth covers /auth/api/v1/...: a {"response": {...}} envelope
	// with no "result" layer.
	FamilyAuth Family = "auth"
)

// Money is the FreshBooks money representation: a decimal amount carried as
// a string so no precision is lost in transit, plus an ISO 4217 code.
type Money struct {
	Amount string `json:"amount"`
	Code   string `json:"code"`
}

// Rat parses the amount into an exact rational. It returns an error rather
// than a lossy float when the amount is not a decimal number.
func (m Money) Rat() (*big.Rat, error) {
	r, ok := new(big.Rat).SetString(m.Amount)
	if !ok {
		return nil, fmt.Errorf("freshbooks: %q is not a decimal amount", m.Amount)
	}
	return r, nil
}

// The three wire formats FreshBooks uses for dates and timestamps.
const (
	// DateLayout is the accounting API's plain date, e.g. "2026-08-23".
	DateLayout = "2006-01-02"
	// DateTimeLayout is the accounting API's account-local timestamp with
	// no zone, e.g. "2026-08-23 17:21:32".
	DateTimeLayout = "2006-01-02 15:04:05"
	// RFC3339Layout is the business-scoped and auth families' timestamp.
	RFC3339Layout = time.RFC3339
)

// Date is a FreshBooks calendar date ("YYYY-MM-DD"). The zero value marshals
// to JSON null and a JSON null unmarshals to the zero value.
type Date struct {
	time.Time
}

// NewDate returns a Date carrying t.
func NewDate(t time.Time) Date { return Date{Time: t} }

// MarshalJSON renders the date as "YYYY-MM-DD", or null when zero.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.Format(DateLayout))
}

// UnmarshalJSON accepts "YYYY-MM-DD", null, or an empty string.
func (d *Date) UnmarshalJSON(data []byte) error {
	s, ok, err := jsonString(data)
	if err != nil {
		return err
	}
	if !ok || s == "" {
		d.Time = time.Time{}
		return nil
	}
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return fmt.Errorf("freshbooks: parsing date %q: %w", s, err)
	}
	d.Time = t
	return nil
}

// DateTime is a FreshBooks timestamp. It accepts all three wire formats the
// API uses and remembers which one it was decoded from, so a value read from
// one family and written back to it round-trips in the same format. A
// DateTime built in Go marshals as RFC 3339 unless InLayout says otherwise.
type DateTime struct {
	time.Time

	layout string
}

// NewDateTime returns a DateTime carrying t, marshalling as RFC 3339.
func NewDateTime(t time.Time) DateTime { return DateTime{Time: t} }

// InLayout returns a copy of dt that marshals using layout, which must be one
// of DateLayout, DateTimeLayout, or RFC3339Layout.
func (dt DateTime) InLayout(layout string) DateTime {
	dt.layout = layout
	return dt
}

// Layout reports the wire format this value marshals as.
func (dt DateTime) Layout() string {
	if dt.layout == "" {
		return RFC3339Layout
	}
	return dt.layout
}

// MarshalJSON renders the timestamp in its remembered layout, or null when
// zero.
func (dt DateTime) MarshalJSON() ([]byte, error) {
	if dt.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(dt.Format(dt.Layout()))
}

// noZoneLayout is a fourth wire format observed in the Projects/Time
// Tracking business-scoped family: an RFC 3339-shaped timestamp with no zone
// offset, e.g. "2019-04-19T18:25:00" (Projects/List Projects, Postman
// example). INFERRED from that example only, not confirmed live; Phase 2's
// batch c added it because the three documented layouts all reject it.
const noZoneLayout = "2006-01-02T15:04:05"

// dateTimeLayouts are tried in order when decoding; the longest, most
// specific format first so "2026-08-23 17:21:32" never matches DateLayout.
var dateTimeLayouts = []string{RFC3339Layout, noZoneLayout, DateTimeLayout, DateLayout}

// UnmarshalJSON accepts RFC 3339, "YYYY-MM-DD HH:MM:SS", "YYYY-MM-DD", the
// zoneless "YYYY-MM-DDTHH:MM:SS" variant, null, or an empty string.
func (dt *DateTime) UnmarshalJSON(data []byte) error {
	s, ok, err := jsonString(data)
	if err != nil {
		return err
	}
	if !ok || s == "" {
		dt.Time, dt.layout = time.Time{}, ""
		return nil
	}
	for _, layout := range dateTimeLayouts {
		t, perr := time.Parse(layout, s)
		if perr == nil {
			dt.Time, dt.layout = t, layout
			return nil
		}
	}
	return fmt.Errorf("freshbooks: parsing timestamp %q: no known FreshBooks layout matches", s)
}

// jsonString decodes a JSON string or null. ok is false for null.
func jsonString(data []byte) (s string, ok bool, err error) {
	if string(data) == "null" {
		return "", false, nil
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return "", false, fmt.Errorf("freshbooks: expected a JSON string, got %s", truncate(string(data), 32))
	}
	return s, true, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// VisState is the FreshBooks visibility state carried by most accounting
// resources.
type VisState int

// The visibility states FreshBooks documents.
const (
	VisStateActive   VisState = 0
	VisStateDeleted  VisState = 1
	VisStateArchived VisState = 2
)

// String names the visibility state, falling back to the numeric form for
// values FreshBooks has not documented.
func (v VisState) String() string {
	switch v {
	case VisStateActive:
		return "active"
	case VisStateDeleted:
		return "deleted"
	case VisStateArchived:
		return "archived"
	default:
		return "vis_state(" + strconv.Itoa(int(v)) + ")"
	}
}

// Sort directions accepted by Sort.
const (
	SortAsc  = "asc"
	SortDesc = "desc"
)

// requestOptions accumulates the effect of the RequestOption values passed
// to a method, before the transport encodes them for a specific family.
type requestOptions struct {
	include []string
	search  map[string]string
	sort    string
	page    int
	perPage int
}

// A RequestOption tunes a single request: which sub-resources to include,
// how to filter, sort, and paginate. The same options work across both API
// families; the transport encodes them the way each family expects.
type RequestOption interface {
	apply(*requestOptions)
}

type optionFunc func(*requestOptions)

func (f optionFunc) apply(o *requestOptions) { f(o) }

// Include asks the API to embed the named sub-resources in the response,
// e.g. Include("lines") on an invoice.
func Include(names ...string) RequestOption {
	return optionFunc(func(o *requestOptions) { o.include = append(o.include, names...) })
}

// Search filters a list request. It is both a RequestOption and the type of
// the Search field on the per-resource list option structs, so the same
// literal works in either position.
type Search map[string]string

func (s Search) apply(o *requestOptions) {
	if o.search == nil {
		o.search = make(map[string]string, len(s))
	}
	for k, v := range s {
		o.search[k] = v
	}
}

// Sort orders a list request by field. dir is SortAsc or SortDesc; anything
// that does not begin with "d" is treated as ascending.
func Sort(field, dir string) RequestOption {
	suffix := "_asc"
	if strings.HasPrefix(strings.ToLower(dir), "d") {
		suffix = "_desc"
	}
	return optionFunc(func(o *requestOptions) { o.sort = field + suffix })
}

// PageNumber selects a 1-based page of a list request.
//
// The design spec calls this option "Page", which collides with the Page[T]
// pagination type; the type keeps the short name because it appears in every
// List signature.
func PageNumber(n int) RequestOption {
	return optionFunc(func(o *requestOptions) { o.page = n })
}

// PerPage sets the page size of a list request.
func PerPage(n int) RequestOption {
	return optionFunc(func(o *requestOptions) { o.perPage = n })
}

// newRequestOptions folds opts into a single requestOptions value.
func newRequestOptions(opts []RequestOption) requestOptions {
	var o requestOptions
	for _, opt := range opts {
		if opt != nil {
			opt.apply(&o)
		}
	}
	return o
}

// values encodes the options as query parameters for fam. The families
// differ only in how filters are spelled: the accounting API takes
// search[field]=value, the business-scoped API takes bare field=value.
func (o requestOptions) values(fam Family) url.Values {
	v := url.Values{}
	for _, name := range o.include {
		v.Add("include[]", name)
	}
	for key, val := range o.search {
		if fam == FamilyAccounting {
			v.Set("search["+key+"]", val)
		} else {
			v.Set(key, val)
		}
	}
	if o.sort != "" {
		v.Set("sort", o.sort)
	}
	if o.page > 0 {
		v.Set("page", strconv.Itoa(o.page))
	}
	if o.perPage > 0 {
		v.Set("per_page", strconv.Itoa(o.perPage))
	}
	return v
}
