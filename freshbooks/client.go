package freshbooks

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

// DefaultBaseURL is the FreshBooks API root. Both API families live behind
// it; the path prefix, not the host, selects the family.
const DefaultBaseURL = "https://api.freshbooks.com"

// Client is a FreshBooks API client. Build one with NewClient and reach the
// API through its service fields. A Client is safe for concurrent use.
type Client struct {
	baseURL     *url.URL
	httpClient  *http.Client
	tokenSource auth.TokenSource
	userAgent   string
	logger      *slog.Logger
	retry       RetryPolicy
	now         func() time.Time

	// Identity resolves the caller's account and business identifiers.
	Identity *IdentityService
	// Clients is the accounting clients resource.
	Clients *ClientsService
	// Contacts is the secondary-contacts resource on a client.
	Contacts *ContactsService
	// Invoices is the invoices resource.
	Invoices *InvoicesService
	// InvoiceProfiles is the recurring-invoice-template resource.
	InvoiceProfiles *InvoiceProfilesService
	// Expenses is the expenses resource.
	Expenses *ExpensesService
	// ExpenseCategories is the expense-categories resource.
	ExpenseCategories *ExpenseCategoriesService
	// Estimates is the estimates and proposals resource.
	Estimates *EstimatesService
	// Payments is the invoice-payments resource.
	Payments *PaymentsService
	// Items is the items and services catalogue resource.
	Items *ItemsService
	// Taxes is the tax-rates resource.
	Taxes *TaxesService
	// Bills is the vendor-bills resource.
	Bills *BillsService
	// BillPayments is the payments-against-bills resource.
	BillPayments *BillPaymentsService
	// BillVendors is the vendors resource.
	BillVendors *BillVendorsService
	// BillableItems is the billable-items resource.
	BillableItems *BillableItemsService
	// CreditNotes is the client-credits resource.
	CreditNotes *CreditNotesService
	// OtherIncome is the other-income resource.
	OtherIncome *OtherIncomeService
	// JournalEntries is the journal-entries resource.
	JournalEntries *JournalEntriesService
	// JournalEntryAccounts is the journal-entry-accounts resource.
	JournalEntryAccounts *JournalEntryAccountsService
	// LedgerAccounts is the chart-of-accounts resource.
	LedgerAccounts *LedgerAccountsService
	// Reports is the reporting resource.
	Reports *ReportsService
	// Systems is the account-system-settings resource.
	Systems *SystemsService
	// Staff is the staff resource.
	Staff *StaffService
	// Tasks is the project-tasks resource.
	Tasks *TasksService
	// Projects is the projects resource.
	Projects *ProjectsService
	// TimeEntries is the time-tracking resource.
	TimeEntries *TimeEntriesService
	// Services is the project-services resource.
	Services *ServicesService
	// ServiceRates is the service-rates resource.
	ServiceRates *ServiceRatesService
	// TeamMembers is the team-members resource.
	TeamMembers *TeamMembersService
	// Retainers is the retainers resource.
	Retainers *RetainersService
	// Callbacks is the webhook-callbacks resource.
	Callbacks *CallbacksService
	// Attachments is the file-uploads resource.
	Attachments *AttachmentsService
	// Images is the image-uploads resource.
	Images *ImagesService
	// Gateways is the payment-gateways resource.
	Gateways *GatewaysService
	// CheckoutLinks is the checkout-links resource.
	CheckoutLinks *CheckoutLinksService
	// PaymentOptions is the invoice-payment-options resource.
	PaymentOptions *PaymentOptionsService
}

// NewClient builds a Client. Without WithTokenSource the client is
// unauthenticated, which is only useful against a fixture server.
func NewClient(opts ...Option) (*Client, error) {
	base, err := url.Parse(DefaultBaseURL)
	if err != nil {
		return nil, err
	}

	c := &Client{
		baseURL:    base,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  "freshbooks-go/" + Version,
		logger:     slog.New(slog.DiscardHandler),
		retry:      DefaultRetryPolicy(),
		now:        time.Now,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	// Redirects must never carry the bearer token to a different host. The
	// standard library only strips it across registered domains, so a
	// redirect between two hosts on the same domain (or two ports on the
	// same host) would otherwise keep it.
	hc := *c.httpClient
	if hc.CheckRedirect == nil {
		hc.CheckRedirect = stripAuthOnCrossHostRedirect
	}
	c.httpClient = &hc

	c.registerServices()
	return c, nil
}

// registerServices wires every service field to c.
func (c *Client) registerServices() {
	c.Identity = &IdentityService{client: c}
	c.Clients = &ClientsService{client: c}
	c.Contacts = &ContactsService{client: c}
	c.Invoices = &InvoicesService{client: c}
	c.InvoiceProfiles = &InvoiceProfilesService{client: c}
	c.Expenses = &ExpensesService{client: c}
	c.ExpenseCategories = &ExpenseCategoriesService{client: c}
	c.Estimates = &EstimatesService{client: c}
	c.Payments = &PaymentsService{client: c}
	c.Items = &ItemsService{client: c}
	c.Taxes = &TaxesService{client: c}
	c.Bills = &BillsService{client: c}
	c.BillPayments = &BillPaymentsService{client: c}
	c.BillVendors = &BillVendorsService{client: c}
	c.BillableItems = &BillableItemsService{client: c}
	c.CreditNotes = &CreditNotesService{client: c}
	c.OtherIncome = &OtherIncomeService{client: c}
	c.JournalEntries = &JournalEntriesService{client: c}
	c.JournalEntryAccounts = &JournalEntryAccountsService{client: c}
	c.LedgerAccounts = &LedgerAccountsService{client: c}
	c.Reports = &ReportsService{client: c}
	c.Systems = &SystemsService{client: c}
	c.Staff = &StaffService{client: c}
	c.Tasks = &TasksService{client: c}
	c.Projects = &ProjectsService{client: c}
	c.TimeEntries = &TimeEntriesService{client: c}
	c.Services = &ServicesService{client: c}
	c.ServiceRates = &ServiceRatesService{client: c}
	c.TeamMembers = &TeamMembersService{client: c}
	c.Retainers = &RetainersService{client: c}
	c.Callbacks = &CallbacksService{client: c}
	c.Attachments = &AttachmentsService{client: c}
	c.Images = &ImagesService{client: c}
	c.Gateways = &GatewaysService{client: c}
	c.CheckoutLinks = &CheckoutLinksService{client: c}
	c.PaymentOptions = &PaymentOptionsService{client: c}
}

// BaseURL reports the API root this client talks to.
func (c *Client) BaseURL() string { return c.baseURL.String() }

// maxRedirects matches net/http's own default redirect budget.
const maxRedirects = 10

// stripAuthOnCrossHostRedirect removes the bearer token when a redirect
// crosses to a different host:port, and caps the redirect chain.
func stripAuthOnCrossHostRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		// Erroring, not ErrUseLastResponse: handing the final 3xx back as
		// the response would surface as a decoded API error carrying a
		// redirect status, which tells the caller nothing true.
		return fmt.Errorf("freshbooks: stopped after %d redirects", maxRedirects)
	}
	if len(via) > 0 && !strings.EqualFold(req.URL.Host, via[len(via)-1].URL.Host) {
		req.Header.Del("Authorization")
	}
	return nil
}

// familyForPath classifies an API path into its family, which decides the
// response envelope, the error shape, and the query encoding.
func familyForPath(path string) Family {
	switch {
	case strings.HasPrefix(path, "/auth/"):
		return FamilyAuth
	case strings.HasPrefix(path, "/accounting/ledger_accounts/"), strings.HasPrefix(path, "/accounting/businesses/"):
		// The ledger-accounts endpoints (chart of accounts + its type
		// taxonomy) return a flat {"data": ...} body, not the accounting
		// envelope, despite living under /accounting/ -- this case must be
		// checked before the general /accounting/ one below. Confirmed
		// against the Postman "List Accounts", "Single Account", and
		// "Create Account" examples (2026-09-01, Phase 2 batch d); no live
		// confirmation yet.
		return FamilyBusiness
	case strings.HasPrefix(path, "/accounting/"):
		return FamilyAccounting
	case strings.HasPrefix(path, "/events/"):
		// Webhook callbacks are account_id-scoped and return the full
		// accounting envelope ({"response":{"result":{"callbacks":[...],
		// "page":...}}}), confirmed against the Postman "List Webhook
		// Callbacks" example (2026-09-01, Phase 2 batch d). Still no live
		// confirmation.
		return FamilyAccounting
	case strings.HasPrefix(path, "/uploads/"):
		// Upload responses are flat ({"image": {...}}, {"attachment":
		// {...}}), no envelope. Confirmed against the Postman "Upload Logo
		// or Proposal Image" example and the FreshBooks expense-attachments
		// docs page (2026-09-01, Phase 2 batch d). No live confirmation yet.
		return FamilyBusiness
	case strings.HasPrefix(path, "/payments/"):
		// Gateway-connection and card-tokenization responses are flat
		// ({"gateway_connections": [...]}, {"credit_card": {...}}), no
		// envelope. Confirmed against the Postman "Get Publishable Key" and
		// "Create Setup Intent" examples (2026-09-01, Phase 2 batch d). No
		// live confirmation yet.
		return FamilyBusiness
	default:
		return FamilyBusiness
	}
}
