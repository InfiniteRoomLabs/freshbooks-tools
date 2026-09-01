package freshbooks

import (
	"context"
	"net/http"
)

// System is an account's system-level settings: currency, address, billing
// cycle, and the misc. account-wide flags the accounting API groups under
// "systems". businessUuid is nullable in the payload; not every account has
// one wired up.
type System struct {
	ID            int64  `json:"id"`
	AccountID     string `json:"accountid"`
	BusinessUUID  string `json:"business_uuid"`
	Active        bool   `json:"active"`
	Country       string `json:"country"`
	CurrencyCode  string `json:"currency_code"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Language      string `json:"language"`
	Timezone      string `json:"timezone"`
	DateFormat    string `json:"date_format"`
	Duration      int    `json:"duration"`
	BillingStatus string `json:"billing_status"`
	BusPhone      string `json:"bus_phone"`
	Fax           string `json:"fax"`
	City          string `json:"city"`
	Code          string `json:"code"`
	Date          string `json:"date"`
}

type systemResponse struct {
	System System `json:"system"`
}

// Get returns account, the single system-settings record for a business
// within accountID. businessID addresses which of the account's businesses
// to read (an account can hold more than one).
//
// inventory: Settings/Systems/Get System
func (s *SystemsService) Get(ctx context.Context, accountID AccountID, businessID BusinessID) (*System, error) {
	var resp systemResponse
	path := "/accounting/account/" + string(accountID) + "/systems/systems/" + businessID.String()
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.System, nil
}
