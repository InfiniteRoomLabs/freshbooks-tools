package freshbooks

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func TestBillPaymentsCreate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] posts the payment payload", func(t *testing.T) {
		var gotPath, gotMethod string
		var gotBody map[string]any
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &gotBody)
			serveFixture(t, http.StatusOK, "accounting", "bill_payments_create")(w, r)
		}))
		bp, err := c.BillPayments.Create(ctx, acct, &BillPaymentCreateRequest{
			BillID:      1141,
			Amount:      Money{Amount: "375.00", Code: "USD"},
			PaymentType: "Check",
			PaidDate:    NewDate(fixedClock()),
		})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/bill_payments/bill_payments" || gotMethod != http.MethodPost {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		inner, _ := gotBody["bill_payment"].(map[string]any)
		if inner["billid"] != float64(1141) || inner["payment_type"] != "Check" {
			t.Fatalf("body = %v", gotBody)
		}
		if bp.ID != 909 || bp.BillID != 1141 {
			t.Fatalf("bill payment = %+v", bp)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.BillPayments.Create(ctx, acct, nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a 422 is ErrValidation", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusUnprocessableEntity, "accounting", "error_422"))
		_, err := c.BillPayments.Create(ctx, acct, &BillPaymentCreateRequest{BillID: 1, PaymentType: "Check"})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestBillPaymentsUpdate(t *testing.T) {
	ctx := context.Background()
	acct := AccountID("ACM123")

	t.Run("[happy] puts the changed fields", func(t *testing.T) {
		var gotPath, gotMethod string
		c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotMethod = r.URL.Path, r.Method
			serveFixture(t, http.StatusOK, "accounting", "bill_payments_update")(w, r)
		}))
		note := "partial payment"
		bp, err := c.BillPayments.Update(ctx, acct, 909, &BillPaymentUpdateRequest{Note: &note})
		if err != nil {
			t.Fatal(err)
		}
		if gotPath != "/accounting/account/ACM123/bill_payments/bill_payments/909" || gotMethod != http.MethodPut {
			t.Fatalf("%s %s", gotMethod, gotPath)
		}
		if bp.Note != "partial payment" {
			t.Fatalf("bill payment = %+v", bp)
		}
	})

	t.Run("[sad] a nil request", func(t *testing.T) {
		c, _ := newTestClient(t, http.NotFoundHandler())
		if _, err := c.BillPayments.Update(ctx, acct, 909, nil); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] a 404 is ErrNotFound", func(t *testing.T) {
		c, _ := newTestClient(t, serveFixture(t, http.StatusNotFound, "accounting", "error_404"))
		note := "x"
		if _, err := c.BillPayments.Update(ctx, acct, 909, &BillPaymentUpdateRequest{Note: &note}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestBillPaymentsRejectUnsafeAccountID(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		acct AccountID
	}{
		{"[sad] a path separator", "a/b"},
		{"[sad] a query delimiter", "a?b"},
		{"[sad] a fragment delimiter", "a#b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}))
			if _, err := c.BillPayments.Create(ctx, tc.acct, &BillPaymentCreateRequest{BillID: 1, PaymentType: "Check"}); err == nil {
				t.Fatal("want an error")
			}
			if called {
				t.Fatal("a request was made with an unsafe account id")
			}
		})
	}
}
