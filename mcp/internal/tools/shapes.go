package tools

import "github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"

// emptyIn is the input type for the handful of tools whose lib method takes
// no scope and no arguments at all (identity_me, identity_whoami,
// identity_applications).
type emptyIn struct{}

// idIn is embedded by every tool input addressing a single resource by its
// numeric id.
type idIn struct {
	ID int64 `json:"id" jsonschema:"the resource id"`
}

// listIn is embedded by every tool input wrapping a List method: the
// filter/page/per_page trio every *ListOptions struct in the lib shares
// (see freshbooks/page.go's listOpts).
type listIn struct {
	Search  map[string]string `json:"search,omitempty" jsonschema:"filter fields as key/value pairs, e.g. {\"status\":\"active\"}"`
	Page    int               `json:"page,omitempty" jsonschema:"1-based page number"`
	PerPage int               `json:"per_page,omitempty" jsonschema:"page size"`
}

// searchOf converts a wire filter map to the lib's Search type, nil when
// empty so an omitted filter never sends an empty search[] query
// parameter. Shared by listIn.search() and by retainersListIn
// (tools_retainers.go), which cannot embed listIn -- RetainerListOptions
// has no Page/PerPage -- but still wants the same conversion.
func searchOf(m map[string]string) freshbooks.Search {
	if len(m) == 0 {
		return nil
	}
	return freshbooks.Search(m)
}

// search converts listIn's wire map to the lib's Search type.
func (l listIn) search() freshbooks.Search { return searchOf(l.Search) }

// reqOpts renders listIn as a []RequestOption, for the handful of methods
// that take raw variadic RequestOptions instead of a *XListOptions struct
// (JournalEntries.Details, JournalEntryAccounts.List).
func (l listIn) reqOpts() []freshbooks.RequestOption {
	var opts []freshbooks.RequestOption
	if s := l.search(); len(s) > 0 {
		opts = append(opts, s)
	}
	if l.Page > 0 {
		opts = append(opts, freshbooks.PageNumber(l.Page))
	}
	if l.PerPage > 0 {
		opts = append(opts, freshbooks.PerPage(l.PerPage))
	}
	return opts
}

// includeIn is embedded by the handful of tool inputs whose lib method
// accepts variadic Include(...) sub-resources: ClientListOptions,
// EstimateListOptions, Invoices.{Create,Update}, InvoiceProfiles.Create.
type includeIn struct {
	Include []string `json:"include,omitempty" jsonschema:"related sub-resources to embed in the response, e.g. [\"lines\"]"`
}

// opts renders Include as the RequestOption slice the lib's variadic
// methods take; empty when no field was set.
func (i includeIn) opts() []freshbooks.RequestOption {
	if len(i.Include) == 0 {
		return nil
	}
	return []freshbooks.RequestOption{freshbooks.Include(i.Include...)}
}

// void adapts a lib method that returns only an error into a Call's (any,
// error) shape: nil passes through as a small acknowledgement, so a
// successful call still carries content instead of an empty result.
func void(err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}
