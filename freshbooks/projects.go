package freshbooks

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
)

// Project is a business's project: the unit time entries, tasks, and
// invoices ultimately roll up to.
//
// GroupID is what List/Create/Update return (a bare id); Get returns the
// expanded Group object instead and leaves GroupID zero -- the captured
// examples for the two shapes never carry both. Get also has a sibling the
// project object does not: a project-scoped "abilities" array (17 entries
// in the captured example), distinct from the business-wide
// ProjectsService.Abilities endpoint's 9-entry list. This method drops it;
// a caller who needs it can call (*Client).Do against
// /projects/business/{id}/projects/{id} and decode both siblings.
type Project struct {
	ID             int64     `json:"id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	DueDate        Date      `json:"due_date"`
	ClientID       int64     `json:"client_id"`
	Internal       bool      `json:"internal"`
	Budget         *int64    `json:"budget"`
	FixedPrice     *string   `json:"fixed_price"`
	Rate           *string   `json:"rate"`
	BillingMethod  *string   `json:"billing_method"`
	ProjectType    string    `json:"project_type"`
	Active         bool      `json:"active"`
	Complete       bool      `json:"complete"`
	Sample         bool      `json:"sample"`
	CreatedAt      DateTime  `json:"created_at"`
	UpdatedAt      DateTime  `json:"updated_at"`
	LoggedDuration *int      `json:"logged_duration"`
	Services       []Service `json:"services"`
	BilledAmount   string    `json:"billed_amount"`
	BilledStatus   string    `json:"billed_status"`
	RetainerID     *int64    `json:"retainer_id"`
	// GroupID is the project's team group id, as List/Create/Update return
	// it.
	GroupID int64 `json:"group_id"`
	// Group carries the expanded team-member and pending-invitation list,
	// as Get returns it instead of GroupID. Left undecoded: the shape
	// (members[], pending_invitations[]) is evidenced but no FreshBooks doc
	// page specs its fields, and no consumer needs it typed yet.
	Group json.RawMessage `json:"group,omitempty"`
}

type projectResponse struct {
	Project Project `json:"project"`
}

// ProjectServiceInput names one service to attach to a project on Create,
// with its billable flag and visibility state.
type ProjectServiceInput struct {
	ID       int64    `json:"id"`
	Billable bool     `json:"billable"`
	VisState VisState `json:"vis_state"`
}

// ProjectCreateRequest is the payload for Create.
type ProjectCreateRequest struct {
	Title       string                `json:"title"`
	ClientID    int64                 `json:"client_id"`
	ProjectType string                `json:"project_type"`
	FixedPrice  string                `json:"fixed_price,omitempty"`
	Rate        string                `json:"rate,omitempty"`
	DueDate     *Date                 `json:"due_date,omitempty"`
	Description string                `json:"description,omitempty"`
	Services    []ProjectServiceInput `json:"services,omitempty"`
}

// Create adds a project.
//
// inventory: Projects/Create Single Project
func (s *ProjectsService) Create(ctx context.Context, businessID BusinessID, req *ProjectCreateRequest) (*Project, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Create needs a request")
	}
	var resp projectResponse
	path := "/projects/business/" + businessID.String() + "/project"
	body := map[string]*ProjectCreateRequest{"project": req}
	if err := s.client.do(ctx, http.MethodPost, path, FamilyBusiness, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Project, nil
}

// Get returns one project by ID. See Project's doc comment: this returns
// the expanded Group object (GroupID stays zero) and drops the sibling
// "abilities" array the captured response carries alongside "project".
//
// inventory: Projects/Single Project
func (s *ProjectsService) Get(ctx context.Context, businessID BusinessID, projectID int64) (*Project, error) {
	var resp projectResponse
	path := fmt.Sprintf("/projects/business/%s/projects/%d", businessID, projectID)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Project, nil
}

type projectsListResponse struct {
	Meta     PageMeta  `json:"meta"`
	Projects []Project `json:"projects"`
}

// ProjectListOptions filters and paginates List. The business family spells
// filters as bare field=value (spec 5.1's STATE AS OF callout, confirmed by
// this batch's TimeEntriesService.List).
type ProjectListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *ProjectListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}

// List returns one page of businessID's projects. Filter it with Search,
// e.g. &ProjectListOptions{Search: Search{"active": "true"}}.
//
// inventory: Projects/List Projects
func (s *ProjectsService) List(ctx context.Context, businessID BusinessID, opts *ProjectListOptions, extra ...RequestOption) (*Page[Project], error) {
	var resp projectsListResponse
	path := "/projects/business/" + businessID.String() + "/projects"
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp, reqOpts...); err != nil {
		return nil, err
	}
	return newPage(resp.Projects, resp.Meta), nil
}

// All walks every page of List.
func (s *ProjectsService) All(ctx context.Context, businessID BusinessID, opts *ProjectListOptions, extra ...RequestOption) iter.Seq2[Project, error] {
	return All(ctx, func(ctx context.Context, page int) (*Page[Project], error) {
		o := ProjectListOptions{Page: page}
		if opts != nil {
			o.Search, o.PerPage = opts.Search, opts.PerPage
		}
		o.PerPage = pageSize(o.PerPage)
		return s.List(ctx, businessID, &o, extra...)
	})
}

// ProjectUpdateRequest is the payload for Update. Every field is optional;
// only set ones are sent.
type ProjectUpdateRequest struct {
	Title       *string `json:"title,omitempty"`
	ClientID    *int64  `json:"client_id,omitempty"`
	DueDate     *Date   `json:"due_date,omitempty"`
	Description *string `json:"description,omitempty"`
	Active      *bool   `json:"active,omitempty"`
	Complete    *bool   `json:"complete,omitempty"`
}

// Update edits a project.
//
// inventory: Projects/Update Project
func (s *ProjectsService) Update(ctx context.Context, businessID BusinessID, projectID int64, req *ProjectUpdateRequest) (*Project, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Update needs a request")
	}
	var resp projectResponse
	path := fmt.Sprintf("/projects/business/%s/project/%d", businessID, projectID)
	body := map[string]*ProjectUpdateRequest{"project": req}
	if err := s.client.do(ctx, http.MethodPut, path, FamilyBusiness, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Project, nil
}

// Delete removes a project. Destructive and irreversible: a CLI or MCP
// surface built on this method must require explicit confirmation and must
// not expose it as an unattended tool.
//
// The Postman collection sources this request from my.freshbooks.com --
// FreshBooks' internal host -- not the public api.freshbooks.com the rest
// of the collection uses; it is also the collection's only request against
// this particular path root (/comments/business/.../project/{id}, not
// /projects/business/... like Get/Update). The inventory tool rewrites the
// host to public; this method implements against that rewritten path.
// INFERRED from that Postman example alone, never confirmed live: if the
// public host answers differently (or not at all), Delete's real shape may
// differ from this.
//
// inventory: Projects/Delete Project
func (s *ProjectsService) Delete(ctx context.Context, businessID BusinessID, projectID int64) error {
	path := fmt.Sprintf("/comments/business/%s/project/%d", businessID, projectID)
	body := map[string]map[string]VisState{"project": {"vis_state": VisStateDeleted}}
	return s.client.do(ctx, http.MethodDelete, path, FamilyBusiness, body, nil)
}

// Ability is one permission flag the current identity holds for a business's
// projects and time tracking.
type Ability struct {
	Name  string `json:"name"`
	Value bool   `json:"value"`
}

type abilitiesResponse struct {
	Abilities []Ability `json:"abilities"`
}

// Abilities lists the current identity's project and time-tracking
// permissions for businessID. It lives on ProjectsService because its path
// is rooted at /projects/business/...; no dedicated "Settings" service
// exists among the 36 pre-declared ones for this Settings-folder key.
//
// inventory: Settings/Abilities/Abilities
func (s *ProjectsService) Abilities(ctx context.Context, businessID BusinessID) ([]Ability, error) {
	var resp abilitiesResponse
	path := "/projects/business/" + businessID.String() + "/abilities"
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Abilities, nil
}

// Threads lists a project's discussion threads. The Postman collection
// carries no response example for this endpoint, so the reply is handed
// back undecoded: the caller can inspect it, but no field is CONFIRMED.
//
// inventory: Projects/List All Messages in Project Discussion
func (s *ProjectsService) Threads(ctx context.Context, businessID BusinessID, projectID int64) ([]map[string]any, error) {
	var resp []map[string]any
	path := fmt.Sprintf("/comments/business/%s/project/%d/threads", businessID, projectID)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// CreateThread starts a new discussion thread on a project with an opening
// message. Like Threads, the response is undecoded: no example exists to
// confirm its shape.
//
// inventory: Projects/Create New Message in Project Discussion
func (s *ProjectsService) CreateThread(ctx context.Context, businessID BusinessID, projectID int64, message string) (map[string]any, error) {
	var resp map[string]any
	path := fmt.Sprintf("/comments/business/%s/project/%d/threads", businessID, projectID)
	body := map[string]map[string]string{"thread": {"message": message}}
	if err := s.client.do(ctx, http.MethodPost, path, FamilyBusiness, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// AddThreadComment posts a reply into an existing discussion thread. Like
// Threads, the response is undecoded: no example exists to confirm its
// shape.
//
// inventory: Projects/Add Comment to Project Discussion Message
func (s *ProjectsService) AddThreadComment(ctx context.Context, businessID BusinessID, threadID int64, content string) (map[string]any, error) {
	var resp map[string]any
	path := fmt.Sprintf("/comments/business/%s/threads/%d/comments", businessID, threadID)
	body := map[string]map[string]string{"comment": {"content": content}}
	if err := s.client.do(ctx, http.MethodPost, path, FamilyBusiness, body, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
