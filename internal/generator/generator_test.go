package generator

import (
	"os"
	"strings"
	"testing"
)

func TestFormatSuffix(t *testing.T) {
	tests := []struct {
		suffix string
		index  int
		want   string
	}{
		{"001", 0, "001"},
		{"001", 1, "002"},
		{"001", 9, "010"},
		{"001", 99, "100"},
		{"ABC01", 0, "ABC01"},
		{"ABC01", 1, "ABC02"},
		{"load_", 0, "load_001"},
		{"load_", 1, "load_002"},
		{"", 0, "001"},
		{"", 1, "002"},
	}

	for _, tc := range tests {
		got := FormatSuffix(tc.suffix, tc.index)
		if got != tc.want {
			t.Errorf("FormatSuffix(%q, %d) = %q, want %q", tc.suffix, tc.index, got, tc.want)
		}
	}
}

func TestGenerator(t *testing.T) {
	data, err := os.ReadFile("../../blrwbl 517_001-4810413900004-260616000002408.xml")
	if err != nil {
		t.Fatalf("failed to read test xml: %v", err)
	}

	gen, err := NewGenerator(Config{
		TemplateXML: string(data),
		Suffix:      "001",
		GLN:         "4819999999999",
	})
	if err != nil {
		t.Fatalf("failed to create generator: %v", err)
	}

	doc0, err := gen.Generate(0)
	if err != nil {
		t.Fatalf("generate 0 error: %v", err)
	}
	if doc0.MessageID != "00000000007712001" {
		t.Errorf("doc0 MessageID = %s, want 00000000007712001", doc0.MessageID)
	}
	if doc0.DocumentID != "00000000007712001" {
		t.Errorf("doc0 DocumentID = %s, want 00000000007712001", doc0.DocumentID)
	}
	if doc0.DeliveryNoteID != "001-4819999999999-260616000002408001" {
		t.Errorf("doc0 DeliveryNoteID = %s, want 001-4819999999999-260616000002408001", doc0.DeliveryNoteID)
	}
	if !strings.Contains(doc0.XML, "<MessageID>00000000007712001</MessageID>") {
		t.Errorf("doc0 XML does not contain updated MessageID")
	}
	if !strings.Contains(doc0.XML, "<MsgSenderID>4819999999999</MsgSenderID>") {
		t.Errorf("doc0 XML does not contain updated MsgSenderID")
	}

	doc1, err := gen.Generate(1)
	if err != nil {
		t.Fatalf("generate 1 error: %v", err)
	}
	if doc1.MessageID != "00000000007712002" {
		t.Errorf("doc1 MessageID = %s, want 00000000007712002", doc1.MessageID)
	}
	if doc1.DeliveryNoteID != "001-4819999999999-260616000002408002" {
		t.Errorf("doc1 DeliveryNoteID = %s, want 001-4819999999999-260616000002408002", doc1.DeliveryNoteID)
	}
}
