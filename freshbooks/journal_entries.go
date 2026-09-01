package freshbooks

import (
	"context"
	"fmt"
	"net/http"
)

// JournalEntryDetail is one debit or credit line of a journal entry.
type JournalEntryDetail struct {
	// SubAccountID identifies the sub-account this line posts to.
	SubAccountID int64 `json:"sub_accountid"`
	// Debit is the debit amount as a decimal string, empty when this line
	// is a credit.
	Debit string `json:"debit,omitempty"`
	// Credit is the credit amount as a decimal string, empty when this line
	// is a debit.
	Credit string `json:"credit,omitempty"`
}

// JournalEntryCreateRequest is the payload for JournalEntriesService.Create.
type JournalEntryCreateRequest struct {
	// Details are the entry's debit/credit lines. They must balance.
	Details []JournalEntryDetail `json:"details"`
	// CurrencyCode is the entry's ISO 4217 currency.
	CurrencyCode string `json:"currency_code"`
	// Description is a free-text note on the entry.
	Description string `json:"description,omitempty"`
	// Name labels the entry.
	Name string `json:"name"`
	// UserEnteredDate is the date the entry is recorded against, "YYYY-MM-DD".
	UserEnteredDate string `json:"user_entered_date"`
}

// JournalEntryDetailResult is one line of a created or fetched journal
// entry, as the API echoes it back with server-assigned IDs.
type JournalEntryDetailResult struct {
	// ID and DetailID both identify this line; the API returns both.
	ID              int64  `json:"id"`
	DetailID        int64  `json:"detailid"`
	SubAccountID    int64  `json:"sub_accountid"`
	Debit           string `json:"debit"`
	Credit          string `json:"credit"`
	CurrencyCode    string `json:"currency_code"`
	Description     string `json:"description"`
	Name            string `json:"name"`
	UserEnteredDate string `json:"user_entered_date"`
}

// JournalEntry is a balanced set of debit/credit lines posted to the ledger.
type JournalEntry struct {
	// ID and EntryID both identify the entry; the API returns both.
	ID              int64                      `json:"id"`
	EntryID         int64                      `json:"entryid"`
	CurrencyCode    string                     `json:"currency_code"`
	Description     string                     `json:"description"`
	Name            string                     `json:"name"`
	UserEnteredDate string                     `json:"user_entered_date"`
	Details         []JournalEntryDetailResult `json:"details"`
}

// JournalEntryDetailEntry is one row of JournalEntriesService.Details: a
// posted debit or credit with the account, sub-account, and source
// transaction it belongs to.
type JournalEntryDetailEntry struct {
	ID                 int64  `json:"id"`
	DetailID           int64  `json:"detailid"`
	Name               string `json:"name"`
	Description        string `json:"description"`
	DetailType         string `json:"detail_type"`
	UserEnteredDate    string `json:"user_entered_date"`
	AccountingSystemID string `json:"accounting_systemid"`
	Debit              *Money `json:"debit"`
	Credit             *Money `json:"credit"`
	Balance            Money  `json:"balance"`
	Account            struct {
		ID                 int64  `json:"id"`
		AccountID          int64  `json:"accountid"`
		AccountName        string `json:"account_name"`
		AccountNumber      string `json:"account_number"`
		AccountType        string `json:"account_type"`
		AccountingSystemID string `json:"accounting_systemid"`
	} `json:"account"`
	SubAccount struct {
		ID                 int64  `json:"id"`
		SubAccountID       int64  `json:"sub_accountid"`
		ParentID           int64  `json:"parentid"`
		AccountSubName     string `json:"account_sub_name"`
		AccountSubNumber   string `json:"account_sub_number"`
		AccountingSystemID string `json:"accounting_systemid"`
	} `json:"sub_account"`
	Entry struct {
		ID                 int64  `json:"id"`
		EntryID            int64  `json:"entryid"`
		AccountingSystemID string `json:"accounting_systemid"`
		ClientID           *int64 `json:"clientid"`
		CreditID           *int64 `json:"creditid"`
		ExpenseID          *int64 `json:"expenseid"`
		IncomeID           *int64 `json:"incomeid"`
		InvoiceID          *int64 `json:"invoiceid"`
		PaymentID          *int64 `json:"paymentid"`
	} `json:"entry"`
}

// Create posts a new, balanced journal entry.
//
// The public host for this operation is INFERRED: the Postman collection's
// only example targets my.freshbooks.com, which the inventory tool rewrites
// to the public path below; this has not been confirmed live.
//
// inventory: Accounting/Journal Entries/Add Journal Entry
func (s *JournalEntriesService) Create(ctx context.Context, acct AccountID, req *JournalEntryCreateRequest) (*JournalEntry, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: JournalEntries.Create needs a request")
	}
	path := "/accounting/account/" + string(acct) + "/journal_entries/journal_entries"
	body := struct {
		JournalEntry *JournalEntryCreateRequest `json:"journal_entry"`
	}{req}
	var resp struct {
		JournalEntry JournalEntry `json:"journal_entry"`
	}
	if err := s.client.do(ctx, http.MethodPost, path, FamilyAccounting, body, &resp); err != nil {
		return nil, err
	}
	return &resp.JournalEntry, nil
}

// Details returns the posted journal-entry detail lines for acct.
//
// inventory: Accounting/Journal Entries/Journal Entry Details
func (s *JournalEntriesService) Details(ctx context.Context, acct AccountID, opts ...RequestOption) ([]JournalEntryDetailEntry, error) {
	path := "/accounting/account/" + string(acct) + "/journal_entries/journal_entry_details"
	var resp struct {
		JournalEntryDetails []JournalEntryDetailEntry `json:"journal_entry_details"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp, opts...); err != nil {
		return nil, err
	}
	return resp.JournalEntryDetails, nil
}

// JournalEntryAccountSub is one sub-account row nested under a
// JournalEntryAccount.
type JournalEntryAccountSub struct {
	ID               int64  `json:"id"`
	SubAccountID     int64  `json:"sub_accountid"`
	ParentID         int64  `json:"parentid"`
	AccountSubName   string `json:"account_sub_name"`
	AccountSubNumber string `json:"account_sub_number"`
	AccountType      string `json:"account_type"`
	CurrencyCode     string `json:"currency_code"`
	Balance          string `json:"balance"`
	Custom           bool   `json:"custom"`
}

// JournalEntryAccount is one ledger account with its running balance, as
// returned by the general-ledger view.
type JournalEntryAccount struct {
	ID            int64                    `json:"id"`
	AccountID     int64                    `json:"accountid"`
	AccountName   string                   `json:"account_name"`
	AccountNumber string                   `json:"account_number"`
	AccountType   string                   `json:"account_type"`
	CurrencyCode  string                   `json:"currency_code"`
	Balance       string                   `json:"balance"`
	SubAccounts   []JournalEntryAccountSub `json:"sub_accounts"`
}

// List returns the general-ledger view for acct: every ledger account's
// running balance and sub-accounts. The Postman collection lists this one
// endpoint under two different names -- "Accounts" under Journal Entries,
// and "General Ledger" under Reports -- both map here.
//
// inventory: Accounting/Journal Entries/Accounts
// inventory: Reports/General Ledger
func (s *JournalEntryAccountsService) List(ctx context.Context, acct AccountID, opts ...RequestOption) ([]JournalEntryAccount, error) {
	path := "/accounting/account/" + string(acct) + "/journal_entry_accounts/journal_entry_accounts"
	var resp struct {
		JournalEntryAccounts []JournalEntryAccount `json:"journal_entry_accounts"`
	}
	if err := s.client.do(ctx, http.MethodGet, path, FamilyAccounting, nil, &resp, opts...); err != nil {
		return nil, err
	}
	return resp.JournalEntryAccounts, nil
}
