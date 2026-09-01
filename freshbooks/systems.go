package freshbooks

import (
	"context"
	"fmt"
	"net/http"
)

// System is an account's system-level settings: currency, billing cycle,
// contact info, and the account-wide flags the accounting API groups under
// "systems". Every field below is modeled from the captured
// Settings/Systems/Get System response.
//
// Fields the captured response returns null for are pointers: BusinessUUID,
// BusinessType, Street2, DiscountID, ReferringURL, LandingURL,
// HeardAboutUsVia, Salutation, NumClients, NumStaff, SizeLimit, SplitToken,
// VATName, VATNumber, and MigratedToSmuxAt. Their Go types beyond
// "nullable" are INFERRED from the field name alone -- the captured example
// never carries a non-null value for any of them, so there is no positive
// evidence for e.g. NumClients being an int rather than a string count.
type System struct {
	ID                int64         `json:"id"`
	SystemID          int64         `json:"systemid"`
	AccountID         string        `json:"accountid"`
	BusinessUUID      *BusinessUUID `json:"business_uuid"`
	BusinessType      *string       `json:"business_type"`
	Active            bool          `json:"active"`
	Name              string        `json:"name"`
	Email             string        `json:"email"`
	InfoEmail         string        `json:"info_email"`
	BusPhone          string        `json:"bus_phone"`
	MobPhone          string        `json:"mob_phone"`
	Fax               string        `json:"fax"`
	Street            string        `json:"street"`
	Street2           *string       `json:"street2"`
	City              string        `json:"city"`
	Province          string        `json:"province"`
	Code              string        `json:"code"`
	Country           string        `json:"country"`
	IP                string        `json:"ip"`
	CurrencyCode      string        `json:"currency_code"`
	Timezone          string        `json:"timezone"`
	TimezoneID        int           `json:"timezoneid"`
	Date              DateTime      `json:"date"`
	Duration          int           `json:"duration"`
	DST               bool          `json:"dst"`
	BillingStatus     string        `json:"billing_status"`
	AutoBill          int           `json:"auto_bill"`
	TestSystem        bool          `json:"test_system"`
	ModernSystem      bool          `json:"modern_system"`
	MasterlockBilling bool          `json:"masterlock_billing"`
	PaymentAmount     Money         `json:"payment_amount"`
	PaymentFrequency  int           `json:"payment_frequency"`
	GSTAmount         Money         `json:"gst_amount"`
	DiscountID        *string       `json:"discountid"`
	ReferralID        string        `json:"referralid"`
	ReferringURL      *string       `json:"referring_url"`
	LandingURL        *string       `json:"landing_url"`
	HeardAboutUsVia   *string       `json:"heard_about_us_via"`
	Salutation        *string       `json:"salutation"`
	NumClients        *int64        `json:"num_clients"`
	NumStaff          *int64        `json:"num_staff"`
	SizeLimit         *int64        `json:"size_limit"`
	SplitToken        *string       `json:"split_token"`
	VATName           *string       `json:"vat_name"`
	VATNumber         *string       `json:"vat_number"`
	MigratedToSmuxAt  *string       `json:"migrated_to_smux_at"`
}

type systemResponse struct {
	System System `json:"system"`
}

func systemPath(acct AccountID, businessID BusinessID) (string, error) {
	if err := pathSegment(string(acct)); err != nil {
		return "", err
	}
	return fmt.Sprintf("/accounting/account/%s/systems/systems/%s", acct, businessID), nil
}

// Get returns account, the single system-settings record for a business
// within accountID. businessID addresses which of the account's businesses
// to read (an account can hold more than one).
//
// inventory: Settings/Systems/Get System
func (s *SystemsService) Get(ctx context.Context, accountID AccountID, businessID BusinessID) (*System, error) {
	path, err := systemPath(accountID, businessID)
	if err != nil {
		return nil, err
	}
	var resp systemResponse
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.System, nil
}
