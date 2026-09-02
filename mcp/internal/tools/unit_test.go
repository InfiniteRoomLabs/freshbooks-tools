package tools

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestResolveScope(t *testing.T) {
	defaults := Scope{AccountID: "ACM-default", BusinessID: 111, BusinessUUID: "uuid-default"}

	t.Run("[happy] no scope embedded never reports missing", func(t *testing.T) {
		_, missing := resolveScope(emptyIn{}, defaults)
		if missing != "" {
			t.Fatalf("missing = %q, want none", missing)
		}
	})

	t.Run("[happy] an override wins over the default", func(t *testing.T) {
		resolved, missing := resolveScope(AcctScope{AccountID: "ACM-override"}, defaults)
		if missing != "" {
			t.Fatalf("missing = %q", missing)
		}
		if resolved.AccountID != "ACM-override" {
			t.Fatalf("AccountID = %q", resolved.AccountID)
		}
	})

	t.Run("[happy] an empty override falls back to the default", func(t *testing.T) {
		resolved, missing := resolveScope(AcctScope{}, defaults)
		if missing != "" {
			t.Fatalf("missing = %q", missing)
		}
		if resolved.AccountID != "ACM-default" {
			t.Fatalf("AccountID = %q", resolved.AccountID)
		}
	})

	t.Run("[sad] account_id missing everywhere is reported", func(t *testing.T) {
		_, missing := resolveScope(AcctScope{}, Scope{})
		if missing != "account_id" {
			t.Fatalf("missing = %q, want account_id", missing)
		}
	})

	t.Run("[sad] business_id missing everywhere is reported", func(t *testing.T) {
		_, missing := resolveScope(BizScope{}, Scope{})
		if missing != "business_id" {
			t.Fatalf("missing = %q, want business_id", missing)
		}
	})

	t.Run("[sad] business_uuid missing everywhere is reported", func(t *testing.T) {
		_, missing := resolveScope(UUIDScope{}, Scope{})
		if missing != "business_uuid" {
			t.Fatalf("missing = %q, want business_uuid", missing)
		}
	})

	t.Run("[edge] the first missing field wins when several are embedded", func(t *testing.T) {
		_, missing := resolveScope(systemsGetIn{}, Scope{})
		if missing != "account_id" {
			t.Fatalf("missing = %q, want account_id (checked before business_id)", missing)
		}
	})

	t.Run("[happy] business_id 0 is a valid override only when non-zero", func(t *testing.T) {
		resolved, missing := resolveScope(BizScope{BusinessID: 42}, defaults)
		if missing != "" || resolved.BusinessID != 42 {
			t.Fatalf("resolved = %+v missing = %q", resolved, missing)
		}
	})
}

func TestErrorText(t *testing.T) {
	t.Run("[happy] a *freshbooks.Error renders the documented JSON shape", func(t *testing.T) {
		err := &freshbooks.Error{StatusCode: 422, Code: 1012, Message: "Invalid field", Field: "email", Family: freshbooks.FamilyAccounting}
		got := errorText(err)
		for _, want := range []string{`"status":422`, `"code":1012`, `"message":"Invalid field"`, `"field":"email"`, `"family":"accounting"`} {
			if !strings.Contains(got, want) {
				t.Errorf("errorText() = %s, want it to contain %s", got, want)
			}
		}
	})

	t.Run("[sad] any other error renders Error()", func(t *testing.T) {
		err := errors.New("boom")
		if got := errorText(err); got != "boom" {
			t.Fatalf("errorText() = %q, want %q", got, "boom")
		}
	})

	t.Run("[edge] a *freshbooks.Error with no code or field omits them", func(t *testing.T) {
		err := &freshbooks.Error{StatusCode: 500, Message: "server error"}
		got := errorText(err)
		if strings.Contains(got, `"code"`) || strings.Contains(got, `"field"`) {
			t.Errorf("errorText() = %s, want code/field omitted when zero", got)
		}
	})
}

func TestErrResult(t *testing.T) {
	r := errResult("oops")
	if !r.IsError {
		t.Fatal("IsError = false")
	}
	if len(r.Content) != 1 {
		t.Fatalf("Content len = %d", len(r.Content))
	}
}

func TestVoid(t *testing.T) {
	t.Run("[happy] nil error yields ok()", func(t *testing.T) {
		out, err := void(nil)
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		m, ok := out.(map[string]bool)
		if !ok || !m["ok"] {
			t.Fatalf("out = %#v", out)
		}
	})

	t.Run("[sad] a non-nil error passes through", func(t *testing.T) {
		want := errors.New("boom")
		out, err := void(want)
		if out != nil {
			t.Fatalf("out = %#v, want nil", out)
		}
		if !errors.Is(err, want) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestManifest(t *testing.T) {
	m := Manifest()
	if len(m) != 168 {
		t.Fatalf("len(Manifest()) = %d, want 168", len(m))
	}
	if !sort.SliceIsSorted(m, func(i, j int) bool { return m[i].Name < m[j].Name }) {
		t.Fatal("Manifest() is not sorted by name")
	}
	for _, tool := range m {
		if tool.Name == "" || tool.Description == "" {
			t.Errorf("tool %+v missing name or description", tool)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %s has a nil InputSchema", tool.Name)
		}
	}
}

func TestRedactApplicationsHandlesNil(t *testing.T) {
	if got := redactApplication(nil); got != nil {
		t.Fatalf("redactApplication(nil) = %v, want nil", got)
	}
	if got := redactApplications(nil); got != nil {
		t.Fatalf("redactApplications(nil) = %v, want nil", got)
	}
}

func TestListInHelpers(t *testing.T) {
	t.Run("[edge] an empty search map yields a nil Search", func(t *testing.T) {
		l := listIn{}
		if l.search() != nil {
			t.Fatalf("search() = %v, want nil", l.search())
		}
		if len(l.reqOpts()) != 0 {
			t.Fatalf("reqOpts() = %v, want empty", l.reqOpts())
		}
	})

	t.Run("[happy] a populated listIn renders search, page, and per_page", func(t *testing.T) {
		l := listIn{Search: map[string]string{"status": "active"}, Page: 2, PerPage: 10}
		if len(l.search()) != 1 {
			t.Fatalf("search() = %v", l.search())
		}
		if len(l.reqOpts()) != 3 {
			t.Fatalf("reqOpts() len = %d, want 3", len(l.reqOpts()))
		}
	})

	t.Run("[edge] an empty includeIn yields no options", func(t *testing.T) {
		if len(includeIn{}.opts()) != 0 {
			t.Fatal("opts() should be empty for a zero includeIn")
		}
	})

	t.Run("[happy] a populated includeIn renders one Include option", func(t *testing.T) {
		if len(includeIn{Include: []string{"lines"}}.opts()) != 1 {
			t.Fatal("opts() should carry one option")
		}
	})
}

func TestUploadInRejectsBadBase64(t *testing.T) {
	_, err := uploadIn{Filename: "x", ContentBase64: "not base64!!"}.reader()
	if err == nil {
		t.Fatal("want an error for invalid base64")
	}
}

// TestMissingScopeIsError exercises the newSpec dispatcher's IsError path
// for a request that supplies no scope override and whose server has no
// default configured -- the "named missing field" contract in
// docs/phases/3/plan.md.
func TestMissingScopeIsError(t *testing.T) {
	upstream, _ := fakeUpstream(t)
	defer upstream.Close()
	clientSession := newTestSession(t, upstream, Scope{}, nil) // no default scope at all
	ctx := context.Background()

	// team_members_invitation_rates embeds only BizScope with no override.
	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "team_members_invitation_rates", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("want IsError for a call with no business_id anywhere")
	}
	if !strings.Contains(errorContentText(result), "business_id") {
		t.Fatalf("error text = %q, want it to name business_id", errorContentText(result))
	}
}

// TestMissingScopeIsErrorForSensitiveSpec is TestMissingScopeIsError's
// counterpart for the newSensitiveSpec dispatch path (registry.go), which
// has its own missing-scope check separate from newSpec's.
func TestMissingScopeIsErrorForSensitiveSpec(t *testing.T) {
	upstream, _ := fakeUpstream(t)
	defer upstream.Close()
	clientSession := newTestSession(t, upstream, Scope{}, nil) // no default scope at all
	ctx := context.Background()

	// payment_options_stripe_create_setup_intent embeds AcctScope with no
	// override.
	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "payment_options_stripe_create_setup_intent",
		Arguments: map[string]any{"payment_method": "pm_test"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("want IsError for a call with no account_id anywhere")
	}
	if !strings.Contains(errorContentText(result), "account_id") {
		t.Fatalf("error text = %q, want it to name account_id", errorContentText(result))
	}
}
