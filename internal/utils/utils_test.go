package utils

import "testing"

func TestRemoveFurigana(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No furigana",
			input:    "こんにちは世界",
			expected: "こんにちは世界",
		},
		{
			name:     "With furigana",
			input:    "今日[きょう]は良い[いい]天気[てんき]です",
			expected: "今日は良い天気です",
		},
		{
			name:     "Multiple brackets in sequence",
			input:    "東京[とうきょう][とうきょう]に行き[い]ました",
			expected: "東京に行きました",
		},
		{
			name:     "Empty brackets",
			input:    "テスト[]です",
			expected: "テストです",
		},
		{
			name:     "Brackets with nested brackets (shouldn't happen but testing edge case)",
			input:    "複雑[ふく[ざつ]]な例",
			expected: "複雑な例",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveFurigana(tt.input)
			if result != tt.expected {
				t.Errorf("RemoveFurigana(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
