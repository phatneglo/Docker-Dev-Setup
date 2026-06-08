package main

import (
	"context"
	"os"
	"testing"
)

type fakeRunner struct {
	output string
	args   []string
}

func (f *fakeRunner) RunSetup(ctx context.Context, args ...string) (string, error) {
	f.args = args
	return f.output, nil
}

func (f *fakeRunner) MailserverStatus(ctx context.Context) bool {
	return true
}

func TestParseEmailList(t *testing.T) {
	users := ParseEmailList("* admin@itbsstudio.com ( 7.0K / ~ ) [0%]\n* user@itbsstudio.com ( 1.0M / 5G ) [2%]\n")
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0].Email != "admin@itbsstudio.com" || users[0].QuotaSet {
		t.Fatalf("unexpected admin parse: %#v", users[0])
	}
	if users[1].Quota != "5G" || users[1].PercentUsed != 2 {
		t.Fatalf("unexpected quota parse: %#v", users[1])
	}
}

func TestReadQuotaFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/dovecot-quotas.cf", []byte("user@itbsstudio.com:1G\n"), 0600); err != nil {
		t.Fatal(err)
	}
	quotas := ReadQuotaFile(dir)
	if quotas["user@itbsstudio.com"] != "1G" {
		t.Fatalf("expected quota from file, got %#v", quotas)
	}
}

func TestReadAccountsFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/postfix-accounts.cf", []byte("user@itbsstudio.com|{SHA512-CRYPT}hash\n"), 0600); err != nil {
		t.Fatal(err)
	}
	accounts := ReadAccountsFile(dir)
	if len(accounts) != 1 || accounts[0] != "user@itbsstudio.com" {
		t.Fatalf("expected account from file, got %#v", accounts)
	}
}

func TestValidateQuota(t *testing.T) {
	for _, value := range []string{"0B", "500M", "1G", "10T", "128k"} {
		if err := validateQuota(value, false); err != nil {
			t.Fatalf("expected quota %s to be valid: %v", value, err)
		}
	}
	for _, value := range []string{"", "1GB", "M500", "-1G", "abc"} {
		if err := validateQuota(value, false); err == nil {
			t.Fatalf("expected quota %s to be invalid", value)
		}
	}
}

func TestAllowedSetupArgs(t *testing.T) {
	allowed := [][]string{
		{"email", "list"},
		{"email", "add", "user@example.com", "secret"},
		{"email", "update", "user@example.com", "secret"},
		{"email", "del", "user@example.com"},
		{"quota", "set", "user@example.com", "1G"},
		{"quota", "del", "user@example.com"},
	}
	for _, args := range allowed {
		if !allowedSetupArgs(args) {
			t.Fatalf("expected args to be allowed: %#v", args)
		}
	}
	blocked := [][]string{
		{"alias", "list"},
		{"email", "restrict", "list", "send"},
		{"quota", "set"},
		{"debug", "show-mail-logs"},
	}
	for _, args := range blocked {
		if allowedSetupArgs(args) {
			t.Fatalf("expected args to be blocked: %#v", args)
		}
	}
}

func TestValidateEmailDomain(t *testing.T) {
	server := NewServer(Config{MailDomain: "itbsstudio.com"}, &fakeRunner{})
	if err := server.validateEmail("user@itbsstudio.com"); err != nil {
		t.Fatalf("expected valid domain: %v", err)
	}
	if err := server.validateEmail("user@yahoo.com"); err == nil {
		t.Fatalf("expected foreign domain to fail")
	}
}
