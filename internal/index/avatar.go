package index

import (
	"time"

	"github.com/openclaw/clawdex/internal/avatar"
	"github.com/openclaw/clawdex/internal/markdown"
	"github.com/openclaw/clawdex/internal/model"
)

func (s Store) SetAvatar(personQuery, imagePath string, now time.Time) (model.Person, error) {
	p, err := s.FindPerson(personQuery)
	if err != nil {
		return model.Person{}, err
	}
	p, err = avatar.SetManual(p, imagePath, now)
	if err != nil {
		return model.Person{}, err
	}
	p.UpdatedAt = now.UTC()
	if err := markdown.WritePerson(p.Path, p); err != nil {
		return model.Person{}, err
	}
	return p, nil
}

func (s Store) ClearAvatar(personQuery string, now time.Time) (model.Person, error) {
	p, err := s.FindPerson(personQuery)
	if err != nil {
		return model.Person{}, err
	}
	p = avatar.Clear(p)
	p.UpdatedAt = now.UTC()
	if err := markdown.WritePerson(p.Path, p); err != nil {
		return model.Person{}, err
	}
	return p, nil
}

func (s Store) RepairAvatarMetadata(person model.Person, now time.Time) (model.Person, bool, error) {
	p, changed, err := avatar.RepairMetadata(person, now)
	if err != nil {
		p = avatar.Clear(person)
		p.UpdatedAt = now.UTC()
		if writeErr := markdown.WritePerson(p.Path, p); writeErr != nil {
			return model.Person{}, false, writeErr
		}
		return p, true, nil
	}
	if !changed {
		return p, false, nil
	}
	if err := markdown.WritePerson(p.Path, p); err != nil {
		return model.Person{}, false, err
	}
	return p, true, nil
}
