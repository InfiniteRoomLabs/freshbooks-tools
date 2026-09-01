package freshbooks

// This file declares one empty service type per resource named in the design
// spec, so the Phase 2 batches only ever add files: each batch fills in the
// methods for its resources without touching client.go and colliding with
// the other batches.

// InvoicesService is the invoices resource.
type InvoicesService struct{ client *Client }

// InvoiceProfilesService is the recurring-invoice-template resource.
type InvoiceProfilesService struct{ client *Client }

// PaymentsService is the invoice-payments resource.
type PaymentsService struct{ client *Client }

// ItemsService is the items and services catalogue resource.
type ItemsService struct{ client *Client }

// BillableItemsService is the billable-items resource.
type BillableItemsService struct{ client *Client }

// OtherIncomeService is the other-income resource.
type OtherIncomeService struct{ client *Client }

// JournalEntriesService is the journal-entries resource.
type JournalEntriesService struct{ client *Client }

// JournalEntryAccountsService is the journal-entry-accounts resource.
type JournalEntryAccountsService struct{ client *Client }

// LedgerAccountsService is the chart-of-accounts resource.
type LedgerAccountsService struct{ client *Client }

// ReportsService is the reporting resource.
type ReportsService struct{ client *Client }

// SystemsService is the account-system-settings resource.
type SystemsService struct{ client *Client }

// StaffService is the staff resource.
type StaffService struct{ client *Client }

// TasksService is the project-tasks resource.
type TasksService struct{ client *Client }

// ProjectsService is the projects resource.
type ProjectsService struct{ client *Client }

// TimeEntriesService is the time-tracking resource.
type TimeEntriesService struct{ client *Client }

// ServicesService is the project-services resource.
type ServicesService struct{ client *Client }

// ServiceRatesService is the service-rates resource.
type ServiceRatesService struct{ client *Client }

// TeamMembersService is the team-members resource.
type TeamMembersService struct{ client *Client }

// RetainersService is the retainers resource.
type RetainersService struct{ client *Client }

// CallbacksService is the webhook-callbacks resource.
type CallbacksService struct{ client *Client }

// AttachmentsService is the file-uploads resource.
type AttachmentsService struct{ client *Client }

// ImagesService is the image-uploads resource.
type ImagesService struct{ client *Client }

// GatewaysService is the payment-gateways resource.
type GatewaysService struct{ client *Client }

// CheckoutLinksService is the checkout-links resource.
type CheckoutLinksService struct{ client *Client }

// PaymentOptionsService is the invoice-payment-options resource.
type PaymentOptionsService struct{ client *Client }
