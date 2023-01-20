package security

import "testing"

func TestGeneratePasswordLength(t *testing.T) {
	password := GeneratePassword()
	if len(password) < 24 {
		t.Errorf("Password is too short, got: '%s'", password)

	}
	if len(password) > 32 {
		t.Errorf("Password is too long, got: '%s'", password)
	}
}

func TestGeneratePasswordRandomness(t *testing.T) {
	password0 := GeneratePassword()
	password1 := GeneratePassword()
	if password0 == password1 {
		t.Errorf("Two Passwords are equals, password0: '%s', password1: '%s'", password0, password1)
	}
}
