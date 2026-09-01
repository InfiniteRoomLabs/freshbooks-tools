package freshbooks

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"
)

func TestBusinessIDString(t *testing.T) {
	if got := BusinessID(8675309).String(); got != "8675309" {
		t.Fatalf("BusinessID.String() = %q, want %q", got, "8675309")
	}
}

func TestMoneyRat(t *testing.T) {
	tests := []struct {
		name    string
		amount  string
		want    string
		wantErr bool
	}{
		{"[happy] plain decimal", "100.00", "100", false},
		{"[edge] fractional cents", "0.005", "1/200", false},
		{"[edge] negative", "-12.50", "-25/2", false},
		{"[sad] not a number", "one hundred", "", true},
		{"[sad] empty", "", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Money{Amount: tc.amount, Code: "USD"}.Rat()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Rat(%q) = %v, want error", tc.amount, r)
				}
				return
			}
			if err != nil {
				t.Fatalf("Rat(%q): %v", tc.amount, err)
			}
			if got := r.RatString(); got != tc.want {
				t.Fatalf("Rat(%q) = %s, want %s", tc.amount, got, tc.want)
			}
		})
	}
}

func TestDateJSON(t *testing.T) {
	t.Run("[happy] round-trips YYYY-MM-DD", func(t *testing.T) {
		var d Date
		if err := json.Unmarshal([]byte(`"2026-08-23"`), &d); err != nil {
			t.Fatal(err)
		}
		if !d.Equal(time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)) {
			t.Fatalf("parsed %v", d.Time)
		}
		b, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != `"2026-08-23"` {
			t.Fatalf("marshalled %s", b)
		}
	})

	t.Run("[edge] null and empty string decode to the zero value", func(t *testing.T) {
		for _, in := range []string{`null`, `""`} {
			var d Date
			if err := json.Unmarshal([]byte(in), &d); err != nil {
				t.Fatalf("%s: %v", in, err)
			}
			if !d.IsZero() {
				t.Fatalf("%s decoded to %v", in, d.Time)
			}
			b, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != "null" {
				t.Fatalf("zero Date marshalled to %s", b)
			}
		}
	})

	t.Run("[sad] wrong layout", func(t *testing.T) {
		var d Date
		if err := json.Unmarshal([]byte(`"23/08/2026"`), &d); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] not a JSON string", func(t *testing.T) {
		var d Date
		if err := json.Unmarshal([]byte(`12345`), &d); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[happy] NewDate", func(t *testing.T) {
		d := NewDate(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
		b, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != `"2026-01-02"` {
			t.Fatalf("marshalled %s", b)
		}
	})
}

func TestDateTimeJSON(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		layout string
		want   time.Time
	}{
		{"[happy] RFC 3339", `"2026-08-22T04:31:37Z"`, RFC3339Layout, time.Date(2026, 8, 22, 4, 31, 37, 0, time.UTC)},
		{"[happy] accounting timestamp", `"2026-08-22 17:21:32"`, DateTimeLayout, time.Date(2026, 8, 22, 17, 21, 32, 0, time.UTC)},
		{"[happy] bare date", `"2026-08-22"`, DateLayout, time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)},
		{"[happy] zoneless RFC 3339 shape (Projects/Time Tracking)", `"2019-04-19T18:25:00"`, noZoneLayout, time.Date(2019, 4, 19, 18, 25, 0, 0, time.UTC)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var dt DateTime
			if err := json.Unmarshal([]byte(tc.in), &dt); err != nil {
				t.Fatal(err)
			}
			if !dt.Equal(tc.want) {
				t.Fatalf("parsed %v, want %v", dt.Time, tc.want)
			}
			if dt.Layout() != tc.layout {
				t.Fatalf("layout %q, want %q", dt.Layout(), tc.layout)
			}
			b, err := json.Marshal(dt)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != tc.in {
				t.Fatalf("round-trip produced %s, want %s", b, tc.in)
			}
		})
	}

	t.Run("[edge] null decodes to zero and marshals back to null", func(t *testing.T) {
		dt := NewDateTime(time.Now())
		if err := json.Unmarshal([]byte(`null`), &dt); err != nil {
			t.Fatal(err)
		}
		if !dt.IsZero() {
			t.Fatalf("decoded %v", dt.Time)
		}
		b, err := json.Marshal(dt)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != "null" {
			t.Fatalf("marshalled %s", b)
		}
	})

	t.Run("[happy] a Go-built value defaults to RFC 3339", func(t *testing.T) {
		dt := NewDateTime(time.Date(2026, 8, 22, 4, 31, 37, 0, time.UTC))
		b, err := json.Marshal(dt)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != `"2026-08-22T04:31:37Z"` {
			t.Fatalf("marshalled %s", b)
		}
	})

	t.Run("[happy] InLayout overrides the wire format", func(t *testing.T) {
		dt := NewDateTime(time.Date(2026, 8, 22, 4, 31, 37, 0, time.UTC)).InLayout(DateTimeLayout)
		b, err := json.Marshal(dt)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != `"2026-08-22 04:31:37"` {
			t.Fatalf("marshalled %s", b)
		}
	})

	t.Run("[sad] unknown layout", func(t *testing.T) {
		var dt DateTime
		if err := json.Unmarshal([]byte(`"Sat, 22 Aug 2026"`), &dt); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("[sad] not a JSON string", func(t *testing.T) {
		var dt DateTime
		if err := json.Unmarshal([]byte(`{}`), &dt); err == nil {
			t.Fatal("want an error")
		}
	})
}

func TestVisStateString(t *testing.T) {
	tests := map[VisState]string{
		VisStateActive:   "active",
		VisStateDeleted:  "deleted",
		VisStateArchived: "archived",
		VisState(9):      "vis_state(9)",
	}
	for in, want := range tests {
		if got := in.String(); got != want {
			t.Fatalf("VisState(%d).String() = %q, want %q", int(in), got, want)
		}
	}
}

func TestRequestOptionValues(t *testing.T) {
	opts := []RequestOption{
		Include("lines", "payments"),
		Search{"status": "paid"},
		Sort("invoice_date", SortDesc),
		PageNumber(2),
		PerPage(25),
		nil,
	}

	tests := []struct {
		name   string
		family Family
		want   string
	}{
		{
			name:   "[happy] accounting spells filters as search[field]",
			family: FamilyAccounting,
			want: url.Values{
				"include[]":      {"lines", "payments"},
				"search[status]": {"paid"},
				"sort":           {"invoice_date_desc"},
				"page":           {"2"},
				"per_page":       {"25"},
			}.Encode(),
		},
		{
			name:   "[happy] business-scoped spells filters as bare fields",
			family: FamilyBusiness,
			want: url.Values{
				"include[]": {"lines", "payments"},
				"status":    {"paid"},
				"sort":      {"invoice_date_desc"},
				"page":      {"2"},
				"per_page":  {"25"},
			}.Encode(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := newRequestOptions(opts).values(tc.family).Encode()
			if got != tc.want {
				t.Fatalf("values = %s, want %s", got, tc.want)
			}
		})
	}

	t.Run("[edge] no options encodes to nothing", func(t *testing.T) {
		if got := newRequestOptions(nil).values(FamilyAccounting).Encode(); got != "" {
			t.Fatalf("values = %q, want empty", got)
		}
	})

	t.Run("[edge] Sort defaults to ascending", func(t *testing.T) {
		got := newRequestOptions([]RequestOption{Sort("created", "")}).values(FamilyAccounting).Encode()
		if got != "sort=created_asc" {
			t.Fatalf("values = %q", got)
		}
	})

	t.Run("[edge] Search merges across options", func(t *testing.T) {
		got := newRequestOptions([]RequestOption{
			Search{"a": "1"}, Search{"b": "2"},
		}).values(FamilyBusiness).Encode()
		if got != "a=1&b=2" {
			t.Fatalf("values = %q", got)
		}
	})

	t.Run("[edge] non-positive page and per_page are omitted", func(t *testing.T) {
		got := newRequestOptions([]RequestOption{PageNumber(0), PerPage(-1)}).values(FamilyAccounting).Encode()
		if got != "" {
			t.Fatalf("values = %q, want empty", got)
		}
	})
}

func TestTruncate(t *testing.T) {
	if got := truncate("abc", 5); got != "abc" {
		t.Fatalf("truncate = %q", got)
	}
	if got := truncate("abcdefgh", 3); got != "abc..." {
		t.Fatalf("truncate = %q", got)
	}
}
