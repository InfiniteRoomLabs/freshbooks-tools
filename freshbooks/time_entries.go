package freshbooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Timer is the running-timer state a time entry may carry.
type Timer struct {
	ID string `json:"id"`
	// IsRunning has no omitempty: it is a field on TimeEntryUpdateRequest
	// too, and a non-pointer bool with omitempty can never send false --
	// which would mean Update could never stop a running timer.
	IsRunning bool `json:"is_running"`
}

// TimeEntry is one logged (or running) block of time against a client,
// project, and optionally a task or service.
type TimeEntry struct {
	ID         int64    `json:"id"`
	IdentityID int64    `json:"identity_id"`
	Timer      *Timer   `json:"timer"`
	IsLogged   bool     `json:"is_logged"`
	StartedAt  DateTime `json:"started_at"`
	CreatedAt  DateTime `json:"created_at"`
	Duration   int      `json:"duration"`
	ClientID   int64    `json:"client_id"`
	ProjectID  int64    `json:"project_id"`
	// PendingClient, PendingProject, and PendingTask carry a not-yet-synced
	// client/project/task name when the entry was logged against one that
	// does not exist as a FreshBooks resource yet (e.g. a quick timer
	// started before the project was created). Always null in the captured
	// examples; typed *string on the strength of the field names alone.
	PendingClient  *string `json:"pending_client"`
	PendingProject *string `json:"pending_project"`
	PendingTask    *string `json:"pending_task"`
	TaskID         *int64  `json:"task_id"`
	ServiceID      *int64  `json:"service_id"`
	Note           string  `json:"note"`
	Active         bool    `json:"active"`
	Billable       bool    `json:"billable"`
	Billed         bool    `json:"billed"`
	Internal       bool    `json:"internal"`
	RetainerID     *int64  `json:"retainer_id"`
}

type timeEntryResponse struct {
	TimeEntry TimeEntry `json:"time_entry"`
}

// timeEntriesListResponse's Meta carries two aggregate fields --
// total_logged and total_unbilled -- that Page[TimeEntry] has no room for
// (Page's four fields are the pagination block every list response shares,
// not a per-resource aggregate). List and Search silently drop them;
// ListWithTotals is the way to reach them.
type timeEntriesListResponse struct {
	Meta        PageMeta    `json:"meta"`
	TimeEntries []TimeEntry `json:"time_entries"`
}

// TimeEntryTotals carries the aggregate fields the time-entries list
// endpoint's meta object holds beside the four pagination keys PageMeta
// already models. TotalLogged and TotalUnbilled are totals across the
// filtered set (not just this page); PerTeamMember and PerClient break the
// same totals down further.
//
// The captured account (freshbooks/testdata/seed/time_entries/list.json)
// has zero time entries, so both breakdown lists are always "[]" there, and
// neither the Postman collection nor the FreshBooks docs page
// (https://www.freshbooks.com/api/time_entries) shows a populated example.
// With no element-shape evidence in either direction, PerTeamMember and
// PerClient stay raw JSON rather than a guessed struct -- INFERRED; see the
// spec 5.1 STATE AS OF 2026-09-03 (Phase 8) callout.
type TimeEntryTotals struct {
	TotalLogged   int `json:"total_logged"`
	TotalUnbilled int `json:"total_unbilled"`
	// PerTeamMember is the meta.total_logged_per_team_member array,
	// undecoded (see the type doc comment).
	PerTeamMember json.RawMessage `json:"total_logged_per_team_member,omitempty"`
	// PerClient is the meta.total_logged_per_client array, undecoded (see
	// the type doc comment).
	PerClient json.RawMessage `json:"total_logged_per_client,omitempty"`
}

// TimeEntriesPage is the result of ListWithTotals: an ordinary
// Page[TimeEntry] plus the totals its meta object carries. The embedded
// Page stays untagged so its own fields promote flat (the MCP and CLI
// output layers key off "items"); Totals carries a tag so the one field
// this type adds is snake_case like every other serialized field in the
// package rather than Go-cased "Totals".
type TimeEntriesPage struct {
	Page[TimeEntry]
	Totals TimeEntryTotals `json:"totals"`
}

// timeEntriesListWithTotalsResponse decodes the same wire response as
// timeEntriesListResponse, but keeps the totals PageMeta drops. Meta embeds
// both PageMeta and TimeEntryTotals so their fields (no keys collide) land
// flat under "meta", matching the wire shape in one decode.
type timeEntriesListWithTotalsResponse struct {
	Meta struct {
		PageMeta
		TimeEntryTotals
	} `json:"meta"`
	TimeEntries []TimeEntry `json:"time_entries"`
}

// TimeEntryListOptions filters and paginates List and Search.
type TimeEntryListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *TimeEntryListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}

func timeEntriesPath(businessID BusinessID) string {
	return "/timetracking/business/" + businessID.String() + "/time_entries"
}

func timeEntryPath(businessID BusinessID, id int64) string {
	return fmt.Sprintf("%s/%d", timeEntriesPath(businessID), id)
}

// list is the shared GET path List and Search both run: same response
// shape, same family, different URL and query.
func (s *TimeEntriesService) list(ctx context.Context, path string, opts []RequestOption) (*Page[TimeEntry], error) {
	var resp timeEntriesListResponse
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp, opts...); err != nil {
		return nil, err
	}
	return newPage(resp.TimeEntries, resp.Meta), nil
}

// List returns one page of businessID's time entries. Filter it with
// opts.Search, e.g. &TimeEntryListOptions{Search: Search{"updated_since":
// t.Format(RFC3339Layout), "include_deleted": "1"}} or
// Search{"started_from": from, "started_to": to} -- the business family
// spells filters as bare field=value (CONFIRMED against the FreshBooks docs
// at https://www.freshbooks.com/api/parameters, 2026-09-01; docs-only, live
// confirmation pending; see the spec 5.1 callout this batch resolved). The
// Postman collection lists this endpoint three times under different
// filter combinations ("List Entries", "Time Entries Updated Since Precise
// Time", "Time Entries for a Given Day"); all three are this one method.
//
// inventory: Time Tracking/List Entries
// inventory: Time Tracking/Time Entries Updated Since Precise Time
// inventory: Time Tracking/Time Entries for a Given Day
func (s *TimeEntriesService) List(ctx context.Context, businessID BusinessID, opts *TimeEntryListOptions, extra ...RequestOption) (*Page[TimeEntry], error) {
	reqOpts := append(opts.opts(), extra...)
	return s.list(ctx, timeEntriesPath(businessID), reqOpts)
}

// ListWithTotals is List's sibling for callers who need the business's
// logged/unbilled totals alongside the page: same one request, same
// endpoint, decoded into TimeEntriesPage instead of Page[TimeEntry] so the
// totals meta carries are not silently dropped. It takes RequestOption
// directly (PageNumber, PerPage, Search, ...) rather than
// *TimeEntryListOptions -- List is untouched and remains the plain-Page
// call.
func (s *TimeEntriesService) ListWithTotals(ctx context.Context, businessID BusinessID, opts ...RequestOption) (*TimeEntriesPage, error) {
	var resp timeEntriesListWithTotalsResponse
	if err := s.client.do(ctx, http.MethodGet, timeEntriesPath(businessID), FamilyBusiness, nil, &resp, opts...); err != nil {
		return nil, err
	}
	page := newPage(resp.TimeEntries, resp.Meta.PageMeta)
	return &TimeEntriesPage{Page: *page, Totals: resp.Meta.TimeEntryTotals}, nil
}

// Search runs a free-text query (e.g. an identity's name) against a
// business's time entries via the dedicated search endpoint. The response
// shape is INFERRED from the sibling list endpoints: the Postman example
// for this request carries the query parameters only, no response body.
//
// inventory: Time Tracking/Time Entries For Employee on Specific Project
func (s *TimeEntriesService) Search(ctx context.Context, businessID BusinessID, query string, opts *TimeEntryListOptions, extra ...RequestOption) (*Page[TimeEntry], error) {
	reqOpts := append([]RequestOption{Search{"q": query, "useSearchEndpoint": "true"}}, opts.opts()...)
	reqOpts = append(reqOpts, extra...)
	return s.list(ctx, timeEntriesPath(businessID)+"/search", reqOpts)
}

// TimeEntryCreateRequest is the payload for Create. IsLogged, Duration,
// StartedAt, and ClientID are required by the API; everything else is
// optional.
type TimeEntryCreateRequest struct {
	IsLogged   bool     `json:"is_logged"`
	Duration   int      `json:"duration"`
	StartedAt  DateTime `json:"started_at"`
	ClientID   int64    `json:"client_id"`
	Note       string   `json:"note,omitempty"`
	ProjectID  *int64   `json:"project_id,omitempty"`
	ServiceID  *int64   `json:"service_id,omitempty"`
	IdentityID *int64   `json:"identity_id,omitempty"`
	Billable   *bool    `json:"billable,omitempty"`
	Billed     *bool    `json:"billed,omitempty"`
}

// Create logs a new time entry.
//
// inventory: Time Tracking/Create a Time Entry
func (s *TimeEntriesService) Create(ctx context.Context, businessID BusinessID, req *TimeEntryCreateRequest) (*TimeEntry, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Create needs a request")
	}
	var resp timeEntryResponse
	body := map[string]*TimeEntryCreateRequest{"time_entry": req}
	if err := s.client.do(ctx, http.MethodPost, timeEntriesPath(businessID), FamilyBusiness, body, &resp); err != nil {
		return nil, err
	}
	return &resp.TimeEntry, nil
}

// TimeEntryUpdateRequest is the payload for Update. Every field is
// optional; only set ones are sent.
type TimeEntryUpdateRequest struct {
	IsLogged  *bool     `json:"is_logged,omitempty"`
	Duration  *int      `json:"duration,omitempty"`
	Note      *string   `json:"note,omitempty"`
	StartedAt *DateTime `json:"started_at,omitempty"`
	ClientID  *int64    `json:"client_id,omitempty"`
	ProjectID *int64    `json:"project_id,omitempty"`
	Timer     *Timer    `json:"timer,omitempty"`
}

// Update edits a time entry.
//
// inventory: Time Tracking/Update a Time Entry
func (s *TimeEntriesService) Update(ctx context.Context, businessID BusinessID, timeEntryID int64, req *TimeEntryUpdateRequest) (*TimeEntry, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Update needs a request")
	}
	var resp timeEntryResponse
	body := map[string]*TimeEntryUpdateRequest{"time_entry": req}
	if err := s.client.do(ctx, http.MethodPut, timeEntryPath(businessID, timeEntryID), FamilyBusiness, body, &resp); err != nil {
		return nil, err
	}
	return &resp.TimeEntry, nil
}

// Delete removes a time entry. Unlike Tasks/Staff, this family has a real
// DELETE verb; there is no soft-delete-via-PUT here.
//
// inventory: Time Tracking/Delete a Time Entry
func (s *TimeEntriesService) Delete(ctx context.Context, businessID BusinessID, timeEntryID int64) error {
	return s.client.do(ctx, http.MethodDelete, timeEntryPath(businessID, timeEntryID), FamilyBusiness, nil, nil)
}
