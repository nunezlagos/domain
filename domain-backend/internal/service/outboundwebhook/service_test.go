package outboundwebhook

import "testing"

func TestValidateURL(t *testing.T) {
	cases := []struct {
		name       string
		url        string
		requireTLS bool
		wantErr    error
	}{
		{"https public ok", "https://hooks.example.com/x", true, nil},
		{"http public ok no tls", "http://hooks.example.com/x", false, nil},
		{"http blocked with tls required", "http://hooks.example.com/x", true, ErrInvalidURL},
		{"localhost blocked", "https://localhost/x", true, ErrSSRF},
		{"127.0.0.1 blocked", "https://127.0.0.1/x", true, ErrSSRF},
		{"10.0.0.1 blocked", "https://10.0.0.1/x", true, ErrSSRF},
		{"192.168.1.1 blocked", "https://192.168.1.1/x", true, ErrSSRF},
		{"172.16.0.1 blocked", "https://172.16.0.1/x", true, ErrSSRF},
		{"172.31.255.255 blocked", "https://172.31.255.255/x", true, ErrSSRF},
		{"172.15.0.1 ok (outside private)", "https://172.15.0.1/x", true, nil},
		{"172.32.0.1 ok (outside private)", "https://172.32.0.1/x", true, nil},
		{".local blocked", "https://api.k8s.local/x", true, ErrSSRF},
		{"169.254 link-local blocked", "https://169.254.169.254/x", true, ErrSSRF},
		{"ftp scheme blocked", "ftp://example.com/x", false, ErrInvalidURL},
		{"empty host", "https:///foo", true, ErrInvalidURL},
		{"bad parse", "://bad", false, ErrInvalidURL},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateURL(tc.url, tc.requireTLS)
			if err != tc.wantErr {
				t.Fatalf("got %v, want %v", err, tc.wantErr)
			}
		})
	}
}
