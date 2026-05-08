package match

import (
	"testing"

	"github.com/openclaw/clawdex/internal/model"
)

func TestCandidateForPrefersEmail(t *testing.T) {
	candidate, ok := CandidateFor(
		model.SourceContact{Name: "Ada", Emails: []model.ContactValue{{Value: "ADA@example.com"}}},
		model.Person{ID: "person_1", Name: "Different", Emails: []model.ContactValue{{Value: "ada@example.com"}}},
	)
	if !ok || candidate.Reason != "email" || candidate.Score != 90 {
		t.Fatalf("candidate = %#v ok=%v", candidate, ok)
	}
}

func TestCandidateForAllReasons(t *testing.T) {
	cases := []struct {
		name    string
		contact model.SourceContact
		person  model.Person
		reason  string
		ok      bool
	}{
		{"external", model.SourceContact{ExternalID: "a1"}, model.Person{ID: "p1", Apple: model.ExternalRef{ID: "a1"}}, "external_id", true},
		{"google", model.SourceContact{ExternalID: "people/c1"}, model.Person{ID: "p1", Google: model.ExternalRef{Resource: "people/c1"}}, "external_id", true},
		{"phone", model.SourceContact{Phones: []model.ContactValue{{Value: "+1 555"}}}, model.Person{ID: "p1", Phones: []model.ContactValue{{Value: "1555"}}}, "phone", true},
		{"name", model.SourceContact{Name: "Ada Lovelace"}, model.Person{ID: "p1", Name: " Ada   Lovelace "}, "name", true},
		{"none", model.SourceContact{Name: "Ada"}, model.Person{ID: "p1", Name: "Grace"}, "", false},
	}
	for _, tt := range cases {
		got, ok := CandidateFor(tt.contact, tt.person)
		if ok != tt.ok || got.Reason != tt.reason {
			t.Fatalf("%s: got %#v ok=%v", tt.name, got, ok)
		}
	}
}
