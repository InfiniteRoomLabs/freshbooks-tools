package freshbooks

import (
	"context"
	"fmt"
	"net/http"
)

// Timer is the running-timer state a time entry may carry.
type Timer struct {
	ID        string `json:"id"`
	IsRunning bool   `json:"is_running,omitempty"`
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
	TaskID     *int64   `json:"task_id"`
	ServiceID  *int64   `json:"service_id"`
	Note       string   `json:"note"`
	Active     bool     `json:"active"`
	Billable   bool     `json:"billable"`
	Billed     bool     `json:"billed"`
	Internal   bool     `json:"internal"`
	RetainerID *int64   `json:"retainer_id"`
}

type timeEntryResponse struct {
	TimeEntry TimeEntry `json:"time_entry"`
}

type timeEntriesListResponse struct {
	Meta struct {
		Total   int `json:"total"`
		Page    int `json:"page"`
		Pages   int `json:"pages"`
		PerPage int `json:"per_page"`
	} `json:"meta"`
	TimeEntries []TimeEntry `json:"time_entries"`
}

// List returns one page of businessID's time entries. Filter it with
// Search, e.g. Search{"updated_since": t.Format(RFC3339Layout),
// "include_deleted": "1"} or Search{"started_from": from, "started_to":
// to} -- the business family spells filters as bare field=value (CONFIRMED
// live against https://www.freshbooks.com/api/parameters, 2026-09-01; see
// the spec 5.1 callout this batch resolved). The Postman collection lists
// this endpoint three times under different filter combinations ("List
// Entries", "Time Entries Updated Since Precise Time", "Time Entries for a
// Given Day"); all three are this one method.
//
// inventory: Time Tracking/List Entries
// inventory: Time Tracking/Time Entries Updated Since Precise Time
// inventory: Time Tracking/Time Entries for a Given Day
func (s *TimeEntriesService) List(ctx context.Context, businessID BusinessID, opts ...RequestOption) (*Page[TimeEntry], error) {
	var resp timeEntriesListResponse
	path := "/timetracking/business/" + businessID.String() + "/time_entries"
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp, opts...); err != nil {
		return nil, err
	}
	return &Page[TimeEntry]{
		Items:   resp.TimeEntries,
		Page:    resp.Meta.Page,
		Pages:   resp.Meta.Pages,
		PerPage: resp.Meta.PerPage,
		Total:   resp.Meta.Total,
	}, nil
}

// Search runs a free-text query (e.g. an identity's name) against a
// business's time entries via the dedicated search endpoint, ordering by
// Sort the same way List does. The response shape is INFERRED from the
// sibling list endpoints: the Postman example for this request carries the
// query parameters only, no response body.
//
// inventory: Time Tracking/Time Entries For Employee on Specific Project
func (s *TimeEntriesService) Search(ctx context.Context, businessID BusinessID, query string, opts ...RequestOption) (*Page[TimeEntry], error) {
	var resp timeEntriesListResponse
	path := "/timetracking/business/" + businessID.String() + "/time_entries/search"
	allOpts := append([]RequestOption{Search{"q": query, "useSearchEndpoint": "true"}}, opts...)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp, allOpts...); err != nil {
		return nil, err
	}
	return &Page[TimeEntry]{
		Items:   resp.TimeEntries,
		Page:    resp.Meta.Page,
		Pages:   resp.Meta.Pages,
		PerPage: resp.Meta.PerPage,
		Total:   resp.Meta.Total,
	}, nil
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
	path := "/timetracking/business/" + businessID.String() + "/time_entries"
	body := map[string]*TimeEntryCreateRequest{"time_entry": req}
	if err := s.client.do(ctx, http.MethodPost, path, FamilyBusiness, body, &resp); err != nil {
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
	path := fmt.Sprintf("/timetracking/business/%s/time_entries/%d", businessID, timeEntryID)
	body := map[string]*TimeEntryUpdateRequest{"time_entry": req}
	if err := s.client.do(ctx, http.MethodPut, path, FamilyBusiness, body, &resp); err != nil {
		return nil, err
	}
	return &resp.TimeEntry, nil
}

// Delete removes a time entry. Unlike Tasks/Staff, this family has a real
// DELETE verb; there is no soft-delete-via-PUT here.
//
// inventory: Time Tracking/Delete a Time Entry
func (s *TimeEntriesService) Delete(ctx context.Context, businessID BusinessID, timeEntryID int64) error {
	path := fmt.Sprintf("/timetracking/business/%s/time_entries/%d", businessID, timeEntryID)
	return s.client.do(ctx, http.MethodDelete, path, FamilyBusiness, nil, nil)
}
