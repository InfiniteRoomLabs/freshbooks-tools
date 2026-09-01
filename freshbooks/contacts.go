package freshbooks

import (
	"context"
	"fmt"
	"net/http"
)

// ContactsService is the secondary-contacts resource on a client. A client
// can have several secondary contacts (e.g. an accounts-payable contact
// distinct from the primary client contact); FreshBooks addresses each one
// directly by its own contactId here, separately from
// ClientsService.RemoveAllSecondaryContacts, which clears every contact on
// a client at once via the client payload.
type ContactsService struct{ client *Client }

// Contact is a client's secondary contact.
type Contact struct {
	// UserID is the contact's identifier, used as contactId in
	// ContactsService.Update and Delete.
	UserID int64 `json:"userid,omitempty"`
	// FirstName, LastName, and Email are the contact's core details.
	FirstName string `json:"fname,omitempty"`
	LastName  string `json:"lname,omitempty"`
	Email     string `json:"email,omitempty"`
	// Phone1 is the contact's phone number.
	Phone1 string `json:"phone1,omitempty"`
	// Face is an avatar image reference, when set.
	Face string `json:"face,omitempty"`
}

type contactEnvelope struct {
	Contact Contact `json:"contact"`
}

// ContactUpdateRequest is the payload for Update. Every field is optional so
// a caller can send only what it means to change.
type ContactUpdateRequest struct {
	FirstName string `json:"fname,omitempty"`
	LastName  string `json:"lname,omitempty"`
	Email     string `json:"email,omitempty"`
	Phone1    string `json:"phone1,omitempty"`
}

func contactPath(acct AccountID, contactID int64) string {
	return fmt.Sprintf("/accounting/account/%s/users/contacts/%d", acct, contactID)
}

// Update changes an existing secondary contact's details. The Postman
// collection carries no example response for this request; the response
// shape here (a "contact" envelope) is INFERRED from the accounting
// family's uniform single-resource response shape, not confirmed live.
//
// inventory: Clients/Edit Secondary Contact ID
func (s *ContactsService) Update(ctx context.Context, acct AccountID, contactID int64, req *ContactUpdateRequest) (*Contact, error) {
	if req == nil {
		return nil, fmt.Errorf("freshbooks: Contacts.Update needs a request")
	}
	body := struct {
		Contact *ContactUpdateRequest `json:"contact"`
	}{req}
	var env contactEnvelope
	if err := s.client.do(ctx, http.MethodPut, contactPath(acct, contactID), FamilyAccounting, body, &env); err != nil {
		return nil, err
	}
	return &env.Contact, nil
}

// Delete removes a single secondary contact. Unlike most soft-deletes in
// this API family, the Postman collection lists this as a real HTTP DELETE,
// not a vis_state-flagged PUT.
//
// inventory: Clients/Delete Secondary  Contact ID
func (s *ContactsService) Delete(ctx context.Context, acct AccountID, contactID int64) error {
	return s.client.do(ctx, http.MethodDelete, contactPath(acct, contactID), FamilyAccounting, nil, nil)
}
