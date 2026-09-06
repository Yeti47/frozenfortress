package scanhandoff

import "testing"

func TestGenerateToken_ProducesUniqueUrlSafeTokens(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 1000; i++ {
		token, err := GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken returned error: %v", err)
		}
		if token == "" {
			t.Fatal("expected a non-empty token")
		}
		if seen[token] {
			t.Fatalf("GenerateToken produced a duplicate token: %s", token)
		}
		seen[token] = true

		for _, r := range token {
			isUrlSafe := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_'
			if !isUrlSafe {
				t.Fatalf("token contains a non-URL-safe character %q: %s", r, token)
			}
		}
	}
}
