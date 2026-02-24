package core

import "testing"

func TestCategoriesForModes_KnownModes(t *testing.T) {
	tests := []struct {
		modes []string
		want  []ModelCategory
	}{
		{[]string{"chat"}, []ModelCategory{CategoryTextGeneration}},
		{[]string{"completion"}, []ModelCategory{CategoryTextGeneration}},
		{[]string{"responses"}, []ModelCategory{CategoryTextGeneration}},
		{[]string{"embedding"}, []ModelCategory{CategoryEmbedding}},
		{[]string{"rerank"}, []ModelCategory{CategoryEmbedding}},
		{[]string{"image_generation"}, []ModelCategory{CategoryImage}},
		{[]string{"image_edit"}, []ModelCategory{CategoryImage}},
		{[]string{"audio_transcription"}, []ModelCategory{CategoryAudio}},
		{[]string{"audio_speech"}, []ModelCategory{CategoryAudio}},
		{[]string{"video_generation"}, []ModelCategory{CategoryVideo}},
		{[]string{"moderation"}, []ModelCategory{CategoryUtility}},
		{[]string{"ocr"}, []ModelCategory{CategoryUtility}},
		{[]string{"search"}, []ModelCategory{CategoryUtility}},
	}

	for _, tt := range tests {
		t.Run(tt.modes[0], func(t *testing.T) {
			got := CategoriesForModes(tt.modes)
			if len(got) != len(tt.want) {
				t.Fatalf("CategoriesForModes(%v) returned %d categories, want %d", tt.modes, len(got), len(tt.want))
			}
			for i, c := range got {
				if c != tt.want[i] {
					t.Errorf("CategoriesForModes(%v)[%d] = %q, want %q", tt.modes, i, c, tt.want[i])
				}
			}
		})
	}
}

func TestCategoriesForModes_MultiMode(t *testing.T) {
	cats := CategoriesForModes([]string{"chat", "image_generation", "audio_speech"})
	want := []ModelCategory{CategoryTextGeneration, CategoryImage, CategoryAudio}
	if len(cats) != len(want) {
		t.Fatalf("got %d categories, want %d", len(cats), len(want))
	}
	for i, c := range cats {
		if c != want[i] {
			t.Errorf("[%d] = %q, want %q", i, c, want[i])
		}
	}
}

func TestCategoriesForModes_Dedup(t *testing.T) {
	// "chat" and "completion" both map to text_generation — should deduplicate
	cats := CategoriesForModes([]string{"chat", "completion"})
	if len(cats) != 1 {
		t.Fatalf("got %d categories, want 1 (deduped)", len(cats))
	}
	if cats[0] != CategoryTextGeneration {
		t.Errorf("got %q, want %q", cats[0], CategoryTextGeneration)
	}
}

func TestCategoriesForModes_UnknownMode(t *testing.T) {
	cats := CategoriesForModes([]string{"unknown_mode"})
	if len(cats) != 0 {
		t.Errorf("CategoriesForModes([\"unknown_mode\"]) = %v, want empty", cats)
	}
}

func TestCategoriesForModes_Empty(t *testing.T) {
	cats := CategoriesForModes(nil)
	if len(cats) != 0 {
		t.Errorf("CategoriesForModes(nil) = %v, want empty", cats)
	}
	cats = CategoriesForModes([]string{})
	if len(cats) != 0 {
		t.Errorf("CategoriesForModes([]) = %v, want empty", cats)
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
