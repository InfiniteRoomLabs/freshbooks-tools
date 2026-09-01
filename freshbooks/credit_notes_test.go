package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestCreditNotesList(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] returns a page of credit notes", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "accounting", "credit_notes_list"))
		page, err := c.CreditNotes.List(ctx, acct, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 || page.Items[0].CreditType != "goodwill" {
			t.Fatalf("page = %+v", page)
		}
	})

	t.Run("[sad] a 429 is ErrRateLimited", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusTooManyRequests, "accounting", "error_429"))
		if _, err := c.CreditNotes.List(ctx, acct, nil); !errors.Is(err, ErrRateLimited) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestCreditNotesAll(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] auto-paginates until a short page", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusOK, "accounting", "credit_notes_list"))
		var got []CreditNote
		for cn, err := range c.CreditNotes.All(ctx, acct, nil) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, cn)
		}
		if len(got) != 1 {
			t.Fatalf("got %d credit notes", len(got))
		}
	})
}

func TestCreditNotesCreate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] posts the credit note payload", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "accounting", "credit_notes_create")(w, r)
		}))
		cn, err := c.CreditNotes.Create(ctx, acct, &CreditNoteWriteRequest{
			ClientID:   55001,
			CreditType: "goodwill",
			Lines:      []CreditNoteLine{{Name: "Credit", Qty: "1", UnitCost: Money{Amount: "150.00", Code: "USD"}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPost {
			t.Fatalf("method = %q", gotMethod)
		}
		inner, _ := gotBody["credit_note"].(map[string]any)
		if inner["credit_type"] != "goodwill" {
			t.Fatalf("body = %v", gotBody)
		}
		if cn.CreditID != 30948 {
			t.Fatalf("credit note = %+v", cn)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.CreditNotes.Create(ctx, acct, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestCreditNotesUpdate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] puts the changed fields", func(t *testing.T) {
		var gotPath, gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			serveFixture(t, http.StatusOK, "accounting", "credit_notes_update")(w, r)
		}))
		cn, err := c.CreditNotes.Update(ctx, acct, 30947, &CreditNoteWriteRequest{Notes: "Updated notes"})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/credit_notes/credit_notes/30947" || gotMethod != http.MethodPut {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		if cn.Notes != "Updated notes" {
			t.Fatalf("credit note = %+v", cn)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.CreditNotes.Update(ctx, acct, 1, nil); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestCreditNotesDelete(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] soft-deletes via a vis_state PUT, not a real DELETE", func(t *testing.T) {
		var gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotMethod = r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"response": {}}`))
		}))
		if err := c.CreditNotes.Delete(ctx, acct, 30947); err != nil {
			t.Fatal(err)
		}
		if gotMethod != http.MethodPut {
			t.Fatalf("method = %q, want PUT", gotMethod)
		}
		inner, _ := gotBody["credit_note"].(map[string]any)
		if inner["vis_state"] != float64(VisStateDeleted) {
			t.Fatalf("body = %v", gotBody)
		}
	})

	t.Run("[sad] a 422 is ErrValidation", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnprocessableEntity, "accounting", "error_422"))
		if err := c.CreditNotes.Delete(ctx, acct, 30947); !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})
}
