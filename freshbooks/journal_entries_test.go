package freshbooks

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestJournalEntriesCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] posts a nested journal_entry body and decodes the created entry", func(t *testing.T) {
		var gotPath string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "journal_entries", "create")(w, r)
		}))

		got, err := c.JournalEntries.Create(ctx, "ACM123", &JournalEntryCreateRequest{
			Details: []JournalEntryDetail{
				{SubAccountID: 635974, Debit: "200"},
				{SubAccountID: 635976, Credit: "200"},
			},
			CurrencyCode:    "USD",
			Name:            "JournalEntry",
			UserEnteredDate: "2019-04-20",
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/journal_entries/journal_entries" {
			t.Fatalf("path = %q", gotPath)
		}
		entry, ok := gotBody["journal_entry"].(map[string]any)
		if !ok || entry["currency_code"] != "USD" {
			t.Fatalf("body = %v", gotBody)
		}
		if got.EntryID != 629310 || len(got.Details) != 2 {
			t.Fatalf("got = %+v", got)
		}
		if got.Details[0].Debit != "200" || got.Details[1].Credit != "200" {
			t.Fatalf("details = %+v", got.Details)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.JournalEntries.Create(ctx, "ACM123", nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] an unbalanced entry error is surfaced", func(t *testing.T) {
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = io.WriteString(w, `{"response": {"errors": [{"errno": 1004, "message": "debits and credits must balance"}]}}`)
		}))
		_, err := c.JournalEntries.Create(ctx, "ACM123", &JournalEntryCreateRequest{Name: "x"})
		if err == nil || !strings.Contains(err.Error(), "balance") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestJournalEntriesDetails(t *testing.T) {
	ctx := context.Background()
	c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "journal_entries", "details"))

	t.Run("[happy] decodes nested account/sub_account/entry", func(t *testing.T) {
		got, err := c.JournalEntries.Details(ctx, "ACM123")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d details", len(got))
		}
		d := got[0]
		if d.Account.AccountName != "Operating Expenses" || d.SubAccount.AccountSubName != "Car & Truck Expenses" {
			t.Fatalf("got = %+v", d)
		}
		if d.Debit == nil || d.Debit.Amount != "70.56" || d.Credit != nil {
			t.Fatalf("debit/credit = %+v / %+v", d.Debit, d.Credit)
		}
		if d.Entry.ExpenseID == nil || *d.Entry.ExpenseID != 1825568 {
			t.Fatalf("entry = %+v", d.Entry)
		}
	})
}

func TestJournalEntryAccountsList(t *testing.T) {
	ctx := context.Background()

	t.Run("[happy] returns accounts with nested sub_accounts", func(t *testing.T) {
		var gotPath string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			serveFixture(t, http.StatusOK, "journal_entry_accounts", "list")(w, r)
		}))
		got, err := c.JournalEntryAccounts.List(ctx, "ACM123")
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/journal_entry_accounts/journal_entry_accounts" {
			t.Fatalf("path = %q", gotPath)
		}
		if len(got) != 2 || got[0].AccountName != "Cash" || len(got[0].SubAccounts) != 1 {
			t.Fatalf("got = %+v", got)
		}
	})

	t.Run("[edge] an account with no sub-accounts", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "journal_entry_accounts", "list"))
		got, err := c.JournalEntryAccounts.List(ctx, "ACM123")
		if err != nil {
			t.Fatal(err)
		}
		if len(got[1].SubAccounts) != 0 {
			t.Fatalf("got = %+v", got[1])
		}
	})
}
