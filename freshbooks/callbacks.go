package freshbooks

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Callback is a webhook subscription: a URI FreshBooks POSTs an event
// notification to when Event fires. It must be verified (see
// CallbacksService.Verify) before FreshBooks will deliver to it.
type Callback struct {
	// CallbackID identifies the subscription. The API also echoes it as
	// "id"; both decode into this field.
	CallbackID int64 `json:"callbackid"`
	// Event is the subscribed event, e.g. "invoice.create", or a bare noun
	// such as "invoice" to match every verb on that resource.
	Event string `json:"event"`
	// URI is the endpoint FreshBooks delivers the event to.
	URI string `json:"uri"`
	// Verified reports whether URI has completed the verification
	// handshake.
	Verified bool `json:"verified"`
}

// UnmarshalJSON accepts either "callbackid" or "id" for the identifier: the
// FreshBooks docs page and the Postman examples use different spellings for
// the same field, and neither is confirmed live.
func (c *Callback) UnmarshalJSON(data []byte) error {
	type alias Callback
	aux := struct {
		ID int64 `json:"id"`
		*alias
	}{alias: (*alias)(c)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if c.CallbackID == 0 {
		c.CallbackID = aux.ID
	}
	return nil
}

// CallbackRegisterRequest is the payload for CallbacksService.Register.
type CallbackRegisterRequest struct {
	// Event is the event to subscribe to.
	Event string `json:"event"`
	// URI is the endpoint FreshBooks will deliver the event to.
	URI string `json:"uri"`
}

// callbackEnvelope is the {"callback": {...}} shape every non-list webhook
// response nests its record in.
type callbackEnvelope struct {
	Callback Callback `json:"callback"`
}

func callbacksPath(acct AccountID, callbackID *int64) string {
	path := "/events/account/" + string(acct) + "/events/callbacks"
	if callbackID != nil {
		path += "/" + strconv.FormatInt(*callbackID, 10)
	}
	return path
}

// Register subscribes uri to event. FreshBooks marks the new subscription
// unverified until Verify completes.
//
// inventory: Webhooks/Register for Callback
func (s *CallbacksService) Register(ctx context.Context, acct AccountID, req *CallbackRegisterRequest) (*Callback, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Callbacks.Register needs a request")
	}
	body := struct {
		Callback *CallbackRegisterRequest `json:"callback"`
	}{req}
	var resp callbackEnvelope
	if err := s.client.do(ctx, http.MethodPost, callbacksPath(acct, nil), FamilyAccounting, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Callback, nil
}

// List returns acct's webhook subscriptions.
//
// inventory: Webhooks/List Webhook Callbacks
func (s *CallbacksService) List(ctx context.Context, acct AccountID, opts ...RequestOption) (*Page[Callback], error) {
	var resp struct {
		Callbacks []Callback `json:"callbacks"`
		PageMeta
	}
	if err := s.client.do(ctx, http.MethodGet, callbacksPath(acct, nil), FamilyAccounting, nil, &resp, opts...); err != nil {
		return nil, err
	}
	return &Page[Callback]{
		Items: resp.Callbacks, Page: resp.PageMeta.Page, Pages: resp.PageMeta.Pages,
		PerPage: resp.PageMeta.PerPage, Total: resp.PageMeta.Total,
	}, nil
}

// Delete removes a webhook subscription. FreshBooks stops delivering to it
// immediately.
//
// inventory: Webhooks/Delete Webhook Callback
func (s *CallbacksService) Delete(ctx context.Context, acct AccountID, callbackID int64) error {
	return s.client.do(ctx, http.MethodDelete, callbacksPath(acct, &callbackID), FamilyAccounting, nil, nil)
}

// Verify completes the verification handshake for a callback, using the
// verifier code FreshBooks delivered to its URI. An unverified callback
// never receives events.
//
// inventory: Webhooks/Verify Webhook Callback
func (s *CallbacksService) Verify(ctx context.Context, acct AccountID, callbackID int64, verifier string) (*Callback, error) {
	body := struct {
		Callback struct {
			Verifier string `json:"verifier"`
		} `json:"callback"`
	}{}
	body.Callback.Verifier = verifier
	var resp callbackEnvelope
	if err := s.client.do(ctx, http.MethodPut, callbacksPath(acct, &callbackID), FamilyAccounting, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Callback, nil
}

// ResendVerification asks FreshBooks to redeliver the verification code for
// an unverified callback. It shares Verify's PUT endpoint but is a distinct
// operation: the request body asks for a resend rather than submitting a
// verifier.
//
// inventory: Webhooks/Resend Verification Code
func (s *CallbacksService) ResendVerification(ctx context.Context, acct AccountID, callbackID int64) error {
	body := struct {
		Callback struct {
			Resend bool `json:"resend"`
		} `json:"callback"`
	}{}
	body.Callback.Resend = true
	return s.client.do(ctx, http.MethodPut, callbacksPath(acct, &callbackID), FamilyAccounting, body, nil)
}
