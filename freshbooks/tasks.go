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
	Updated     DateTime `json:"updated"`
	VisState    VisState `json:"vis_state"`
}

type taskResponse struct {
	Task Task `json:"task"`
}

// TaskListOptions filters and paginates List.
type TaskListOptions struct {
	Search  Search
	Page    int
	PerPage int
}

func (o *TaskListOptions) opts() []RequestOption {
	if o == nil {
		return nil
	}
	return listOpts(o.Search, o.Page, o.PerPage)
}

func tasksPath(acct AccountID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/projects/tasks", acct), nil
}

func taskPath(acct AccountID, id int64) (string, error) {
	base, err := tasksPath(acct)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%d", base, id), nil
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
	path, err := tasksPath(accountID)
	if err != nil {
		return nil, err
	}
	var resp taskResponse
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
	path, err := taskPath(accountID, taskID)
	if err != nil {
		return nil, err
	}
	var resp taskResponse
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Task, nil
}

type tasksListResponse struct {
	Tasks []Task `json:"tasks"`
	PageMeta
}

// List returns one page of accountID's tasks.
//
// inventory: Projects/Tasks/List Tasks
func (s *TasksService) List(ctx context.Context, accountID AccountID, opts *TaskListOptions, extra ...RequestOption) (*Page[Task], error) {
	path, err := tasksPath(accountID)
	if err != nil {
		return nil, err
	}
	var resp tasksListResponse
	reqOpts := append(opts.opts(), extra...)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp, reqOpts...); err != nil {
		return nil, err
	}
	return newPage(resp.Tasks, resp.PageMeta), nil
}

// All walks every page of List.
func (s *TasksService) All(ctx context.Context, accountID AccountID, opts *TaskListOptions, extra ...RequestOption) iter.Seq2[Task, error] {
	return All(ctx, func(ctx context.Context, page int) (*Page[Task], error) {
		o := TaskListOptions{Page: page}
		if opts != nil {
			o.Search, o.PerPage = opts.Search, opts.PerPage
		}
		o.PerPage = pageSize(o.PerPage)
		return s.List(ctx, accountID, &o, extra...)
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
	path, err := taskPath(accountID, taskID)
	if err != nil {
		return nil, err
	}
	var resp taskResponse
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
	path, err := taskPath(accountID, taskID)
	if err != nil {
		return err
	}
	return s.client.softDelete(ctx, path, "task")
}
