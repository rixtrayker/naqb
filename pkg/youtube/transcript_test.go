package youtube

import (
	"testing"
)

func TestExtractVideoID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "standard watch URL",
			input: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			want:  "dQw4w9WgXcQ",
		},
		{
			name:  "short youtu.be URL",
			input: "https://youtu.be/dQw4w9WgXcQ",
			want:  "dQw4w9WgXcQ",
		},
		{
			name:  "embed URL",
			input: "https://www.youtube.com/embed/dQw4w9WgXcQ",
			want:  "dQw4w9WgXcQ",
		},
		{
			name:  "raw video ID",
			input: "dQw4w9WgXcQ",
			want:  "dQw4w9WgXcQ",
		},
		{
			name:    "invalid URL",
			input:   "https://example.com/video",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractVideoID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ExtractVideoID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("ExtractVideoID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
