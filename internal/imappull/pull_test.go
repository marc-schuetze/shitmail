package imappull

import (
	"reflect"
	"testing"
)

var defaultHeaders = []string{"X-Original-To", "Delivered-To", "X-Envelope-To", "X-Forwarded-To"}

func TestEnvelope(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		want  []string
		from  string
	}{
		{
			name: "X-Original-To wins over the catch-all Delivered-To",
			raw: "From: Sender <boss@example.com>\r\n" +
				"Delivered-To: spool@burner.example\r\n" +
				"X-Original-To: slack1-12345@burner.example\r\n" +
				"To: undisclosed-recipients:;\r\n" +
				"Subject: hi\r\n\r\nbody\r\n",
			want: []string{"slack1-12345@burner.example"},
			from: "boss@example.com",
		},
		{
			name: "Delivered-To used when X-Original-To is absent",
			raw: "From: a@example.com\r\n" +
				"Delivered-To: <shop-abcde@burner.example>\r\n\r\nbody\r\n",
			want: []string{"shop-abcde@burner.example"},
			from: "a@example.com",
		},
		{
			name: "falls back to To and Cc, foreign domains dropped",
			raw: "From: a@example.com\r\n" +
				"To: Bob <bob@elsewhere.test>, Burner <NEWS-ABCDE@BURNER.EXAMPLE>\r\n" +
				"Cc: other-abcde@burner.example\r\n\r\nbody\r\n",
			want: []string{"news-abcde@burner.example", "other-abcde@burner.example"},
			from: "a@example.com",
		},
		{
			name: "nothing at our domain",
			raw:  "From: a@example.com\r\nTo: bob@elsewhere.test\r\n\r\nbody\r\n",
			want: nil,
			from: "a@example.com",
		},
		{
			name: "unparseable message",
			raw:  "not a message at all",
			want: nil,
			from: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, from := envelope([]byte(tc.raw), defaultHeaders, "burner.example")
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("recipients = %v, want %v", got, tc.want)
			}
			if from != tc.from {
				t.Errorf("from = %q, want %q", from, tc.from)
			}
		})
	}
}
