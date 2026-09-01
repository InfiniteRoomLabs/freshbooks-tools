package freshbooks

import (
	"context"
	"fmt"
	"iter"
	"net/http"
)

// Task is an accounting-family billable task, the line item projects and
// time entries bill against.
type Task struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Rate        Money    `json:"rate"`
	Billable    bool     `json:"billable"`
	Tax1        int      `json:"tax1"`
	Tax2        int      `json:"tax2"`
	Updated     string   `json:"updated"`
	VisState    VisState `json:"vis_state"`
}

type taskResponse struct {
	Task Task `json:"task"`
}

// TaskCreateRequest is the payload for Create.
type TaskCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Rate        *Money `json:"rate,omitempty"`
	Billable    *bool  `json:"billable,omitempty"`
}

// Create adds a task.
//
// inventory: Projects/Tasks/Create Task
func (s *TasksService) Create(ctx context.Context, accountID AccountID, req *TaskCreateRequest) (*Task, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Create needs a request")
	}
	var resp taskResponse
	path := "/accounting/account/" + string(accountID) + "/projects/tasks"
	body := map[string]*TaskCreateRequest{"task": req}
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Task, nil
}

// Get returns one task by ID.
//
// inventory: Projects/Tasks/Single Task
func (s *TasksService) Get(ctx context.Context, accountID AccountID, taskID int64) (*Task, error) {
	var resp taskResponse
	path := fmt.Sprintf("/accounting/account/%s/projects/tasks/%d", accountID, taskID)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Task, nil
}

type tasksListResponse struct {
	Page    int    `json:"page"`
	Pages   int    `json:"pages"`
	PerPage int    `json:"per_page"`
	Total   int    `json:"total"`
	Tasks   []Task `json:"tasks"`
}

// List returns one page of accountID's tasks.
//
// inventory: Projects/Tasks/List Tasks
func (s *TasksService) List(ctx context.Context, accountID AccountID, opts ...RequestOption) (*Page[Task], error) {
	var resp tasksListResponse
	path := "/accounting/account/" + string(accountID) + "/projects/tasks"
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp, opts...); err != nil {
		return nil, err
	}
	return &Page[Task]{
		Items:   resp.Tasks,
		Page:    resp.Page,
		Pages:   resp.Pages,
		PerPage: resp.PerPage,
		Total:   resp.Total,
	}, nil
}

// All walks every page of List.
func (s *TasksService) All(ctx context.Context, accountID AccountID, opts ...RequestOption) iter.Seq2[Task, error] {
	return All(ctx, func(ctx context.Context, page int) (*Page[Task], error) {
		return s.List(ctx, accountID, append([]RequestOption{PageNumber(page)}, opts...)...)
	})
}

// TaskUpdateRequest is the payload for Update. Every field is optional; only
// set ones are sent.
type TaskUpdateRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Rate        *Money  `json:"rate,omitempty"`
	Billable    *bool   `json:"billable,omitempty"`
}

// Update edits a task.
//
// inventory: Projects/Tasks/Update Task
func (s *TasksService) Update(ctx context.Context, accountID AccountID, taskID int64, req *TaskUpdateRequest) (*Task, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Update needs a request")
	}
	var resp taskResponse
	path := fmt.Sprintf("/accounting/account/%s/projects/tasks/%d", accountID, taskID)
	body := map[string]*TaskUpdateRequest{"task": req}
	if err := s.client.do(ctx, http.MethodPut, path, FamilyAccounting, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Task, nil
}

// Delete soft-deletes a task by setting its vis_state; the accounting family
// has no task-specific DELETE verb.
//
// inventory: Projects/Tasks/Delete Task
func (s *TasksService) Delete(ctx context.Context, accountID AccountID, taskID int64) error {
	path := fmt.Sprintf("/accounting/account/%s/projects/tasks/%d", accountID, taskID)
	body := map[string]map[string]VisState{"task": {"vis_state": VisStateDeleted}}
	return s.client.do(ctx, http.MethodPut, path, FamilyAccounting, body, nil)
}
