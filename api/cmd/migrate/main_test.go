package main

import (
	"strings"
	"testing"
)

func TestSplitSQLKeepsDollarQuotedFunctionTogether(t *testing.T) {
	sql := `
CREATE TABLE example_items (
    id VARCHAR(64) PRIMARY KEY
);

CREATE OR REPLACE FUNCTION example_set_updated_at()
RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_example_items_set_updated_at
BEFORE UPDATE ON example_items
FOR EACH ROW
EXECUTE FUNCTION example_set_updated_at();
`

	statements := splitSQL(sql)
	if len(statements) != 3 {
		t.Fatalf("expected 3 statements, got %d: %#v", len(statements), statements)
	}
	if want := "CREATE OR REPLACE FUNCTION"; !strings.Contains(statements[1], want) {
		t.Fatalf("expected second statement to contain %q, got %q", want, statements[1])
	}
	if want := "NEW.updated_at = now();"; !strings.Contains(statements[1], want) {
		t.Fatalf("expected function body to keep semicolon, got %q", statements[1])
	}
}
