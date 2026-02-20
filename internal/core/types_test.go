package core

import "testing"

func TestCategoryForMode_KnownModes(t *testing.T) {
	tests := []struct {
		mode string
		want ModelCategory
	}{
		{"chat", CategoryTextGeneration},
		{"completion", CategoryTextGeneration},
		{"responses", CategoryTextGeneration},
		{"embedding", CategoryEmbedding},
		{"rerank", CategoryEmbedding},
		{"image_generation", CategoryImage},
		{"image_edit", CategoryImage},
		{"audio_transcription", CategoryAudio},
		{"audio_speech", CategoryAudio},
		{"video_generation", CategoryVideo},
		{"moderation", CategoryUtility},
		{"ocr", CategoryUtility},
		{"search", CategoryUtility},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			got := CategoryForMode(tt.mode)
			if got != tt.want {
				t.Errorf("CategoryForMode(%q) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestCategoryForMode_UnknownMode(t *testing.T) {
	got := CategoryForMode("unknown_mode")
	if got != "" {
		t.Errorf("CategoryForMode(\"unknown_mode\") = %q, want empty string", got)
	}
}

func TestCategoryForMode_EmptyMode(t *testing.T) {
	got := CategoryForMode("")
	if got != "" {
		t.Errorf("CategoryForMode(\"\") = %q, want empty string", got)
	}
}

func TestAllCategories_Order(t *testing.T) {
	cats := AllCategories()

	expected := []ModelCategory{
		CategoryAll,
		CategoryTextGeneration,
		CategoryEmbedding,
		CategoryImage,
		CategoryAudio,
		CategoryVideo,
		CategoryUtility,
	}

	if len(cats) != len(expected) {
		t.Fatalf("AllCategories() returned %d categories, want %d", len(cats), len(expected))
	}

	for i, cat := range cats {
		if cat != expected[i] {
			t.Errorf("AllCategories()[%d] = %q, want %q", i, cat, expected[i])
		}
	}
}
