package main

import "encoding/json"

// Collection is the subset of the Postman v2.1 collection schema this tool
// reads: a tree of folders and requests under the top-level "item" array.
type Collection struct {
	Item []Item `json:"item"`
}

// Item is either a folder (Item is non-empty, Request is nil) or a leaf
// request (Request is non-nil).
type Item struct {
	Name     string     `json:"name"`
	Item     []Item     `json:"item,omitempty"`
	Request  *Request   `json:"request,omitempty"`
	Response []Response `json:"response,omitempty"`
}

// Request is the subset of a Postman request object this tool reads.
type Request struct {
	Method string `json:"method"`
	URL    URL    `json:"url"`
	Body   *Body  `json:"body,omitempty"`
}

// Body is a Postman request body. Only "raw" mode bodies are read; other
// modes (formdata, urlencoded, ...) are treated as having no captured body.
type Body struct {
	Mode string `json:"mode"`
	Raw  string `json:"raw"`
}

// Response is a saved example response attached to a request.
type Response struct {
	Name string `json:"name"`
	Code int    `json:"code"`
	Body string `json:"body"`
}

// QueryParam is one entry of a Postman URL's structured query array.
type QueryParam struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
}

// URL is a Postman request URL. The Postman schema allows a request's "url"
// to be either a plain string or an object with a "raw" field and a
// structured "query" array; URL accepts both. FromObject records which form
// was seen, since the two forms source query parameters differently: object
// form uses the structured Query array verbatim, string form derives query
// parameters by parsing Raw's query string.
type URL struct {
	Raw        string
	Query      []QueryParam
	FromObject bool
}

// UnmarshalJSON implements json.Unmarshaler, accepting either a JSON string
// or a Postman URL object.
func (u *URL) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		u.Raw = s
		u.Query = nil
		u.FromObject = false
		return nil
	}

	var obj struct {
		Raw   string       `json:"raw"`
		Query []QueryParam `json:"query"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	u.Raw = obj.Raw
	u.Query = obj.Query
	u.FromObject = true
	return nil
}
