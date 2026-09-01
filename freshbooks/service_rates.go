package freshbooks

import (
	"context"
	"fmt"
	"net/http"
)

// ServiceRate is the hourly rate a business has set for one service.
type ServiceRate struct {
	Rate       string `json:"rate"`
	ServiceID  int64  `json:"service_id"`
	BusinessID int64  `json:"business_id"`
}

type serviceRateResponse struct {
	ServiceRate ServiceRate `json:"service_rate"`
}

// Get returns businessID's rate for one service.
//
// inventory: Settings/Items and Services/Get a Single Service Rate
func (s *ServiceRatesService) Get(ctx context.Context, businessID BusinessID, serviceID int64) (*ServiceRate, error) {
	var resp serviceRateResponse
	path := fmt.Sprintf("/comments/business/%s/service/%d/rate", businessID, serviceID)
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.ServiceRate, nil
}

type serviceRatesListResponse struct {
	ServiceRates []ServiceRate `json:"service_rates"`
}

// List returns every service rate set for businessID -- every service's
// rate at once, not one project's. For a single project's rate on a
// service, see UpdateProjectRate.
//
// inventory: Projects/Service Rates
func (s *ServiceRatesService) List(ctx context.Context, businessID BusinessID) ([]ServiceRate, error) {
	var resp serviceRatesListResponse
	path := "/comments/business/" + businessID.String() + "/service_rates"
	if err := s.client.do(ctx, http.MethodGet, path, FamilyBusiness, nil, &resp); err != nil {
		return nil, err
	}
	return resp.ServiceRates, nil
}

// ProjectServiceRate is one project's rate for one of its services --
// distinct from ServiceRate, which is business-wide.
type ProjectServiceRate struct {
	Rate       string `json:"rate"`
	ServiceID  int64  `json:"service_id"`
	BusinessID int64  `json:"business_id"`
	ProjectID  int64  `json:"project_id"`
}

type projectServiceRateResponse struct {
	ProjectServiceRate ProjectServiceRate `json:"project_service_rate"`
}

// UpdateProjectRate sets the rate a specific project charges for a specific
// service, overriding the business-wide ServiceRate for that combination.
//
// inventory: Projects/Update Service Rates
func (s *ServiceRatesService) UpdateProjectRate(ctx context.Context, businessID BusinessID, projectID, serviceID int64, rate string) (*ProjectServiceRate, error) {
	var resp projectServiceRateResponse
	path := fmt.Sprintf("/comments/business/%s/project/%d/service/%d/rate", businessID, projectID, serviceID)
	body := map[string]map[string]any{
		"project_service_rate": {"rate": rate, "service_id": serviceID, "project_id": projectID},
	}
	if err := s.client.do(ctx, http.MethodPut, path, FamilyBusiness, body, &resp); err != nil {
		return nil, err
	}
	return &resp.ProjectServiceRate, nil
}
