package services

import (
	"strings"
	"testing"

	"github.com/faeln1/go-whatsapp-api/internal/domain/message"
)

func TestBuildVCard(t *testing.T) {
	tests := []struct {
		name    string
		contact message.ContactEntry
		want    []string
	}{
		{
			name: "full contact",
			contact: message.ContactEntry{
				FullName:     "John Doe",
				PhoneNumber:  "5511999999999",
				Organization: "ACME Corp",
				Email:        "john@example.com",
				URL:          "https://example.com",
			},
			want: []string{
				"BEGIN:VCARD",
				"VERSION:3.0",
				"FN:John Doe",
				"N:John Doe",
				"TEL;type=CELL;waid=5511999999999:+5511999999999",
				"ORG:ACME Corp;",
				"EMAIL:john@example.com",
				"URL:https://example.com",
				"END:VCARD",
			},
		},
		{
			name: "minimal contact",
			contact: message.ContactEntry{
				FullName:    "Jane Smith",
				PhoneNumber: "5521988888888",
			},
			want: []string{
				"BEGIN:VCARD",
				"VERSION:3.0",
				"FN:Jane Smith",
				"N:Jane Smith",
				"TEL;type=CELL;waid=5521988888888:+5521988888888",
				"END:VCARD",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildVCard(tt.contact)
			for _, expected := range tt.want {
				if !strings.Contains(got, expected) {
					t.Errorf("buildVCard() missing expected string %q\nGot:\n%s", expected, got)
				}
			}
		})
	}
}
