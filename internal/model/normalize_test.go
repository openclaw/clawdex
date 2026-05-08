package model

import "testing"

func TestSlugStable(t *testing.T) {
	if got := Slug("Sally O'Malley"); got != "sally-o-malley" {
		t.Fatalf("Slug = %q", got)
	}
	if got := NormalizePhone("+1 (415) 734-7847"); got != "14157347847" {
		t.Fatalf("NormalizePhone = %q", got)
	}
	if got := NormalizePhone("0043 664 104 2436"); got != "436641042436" {
		t.Fatalf("NormalizePhone 00 = %q", got)
	}
	if got := NormalizeEmail(" ADA@Example.COM "); got != "ada@example.com" {
		t.Fatalf("NormalizeEmail = %q", got)
	}
	if got := NormalizeName(" Ada   Lovelace "); got != "ada lovelace" {
		t.Fatalf("NormalizeName = %q", got)
	}
	if got := PathSlug("/tmp/ada/person.md"); got != "ada" {
		t.Fatalf("PathSlug = %q", got)
	}
	if got := Slug("***"); got != "person" {
		t.Fatalf("empty Slug = %q", got)
	}
}
