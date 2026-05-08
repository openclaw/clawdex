package match

import "github.com/openclaw/clawdex/internal/model"

type Candidate struct {
	PersonID string `json:"person_id"`
	Reason   string `json:"reason"`
	Score    int    `json:"score"`
}

func CandidateFor(contact model.SourceContact, p model.Person) (Candidate, bool) {
	if contact.ExternalID != "" && (p.Apple.ID == contact.ExternalID || p.Google.Resource == contact.ExternalID) {
		return Candidate{PersonID: p.ID, Reason: "external_id", Score: 100}, true
	}
	for _, email := range contact.Emails {
		for _, existing := range p.Emails {
			if model.NormalizeEmail(email.Value) != "" && model.NormalizeEmail(email.Value) == model.NormalizeEmail(existing.Value) {
				return Candidate{PersonID: p.ID, Reason: "email", Score: 90}, true
			}
		}
	}
	for _, phone := range contact.Phones {
		for _, existing := range p.Phones {
			if model.NormalizePhone(phone.Value) != "" && model.NormalizePhone(phone.Value) == model.NormalizePhone(existing.Value) {
				return Candidate{PersonID: p.ID, Reason: "phone", Score: 80}, true
			}
		}
	}
	if model.NormalizeName(contact.Name) != "" && model.NormalizeName(contact.Name) == model.NormalizeName(p.Name) {
		return Candidate{PersonID: p.ID, Reason: "name", Score: 40}, true
	}
	return Candidate{}, false
}
