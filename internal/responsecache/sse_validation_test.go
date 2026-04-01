package responsecache

import "testing"

func TestValidateCacheableSSE(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  []byte
		want bool
	}{
		{
			name: "valid chat stream",
			raw: []byte(
				"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\n" +
					"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" +
					"data: [DONE]\n\n",
			),
			want: true,
		},
		{
			name: "rejects truncated stream without done",
			raw: []byte(
				"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\n",
			),
			want: false,
		},
		{
			name: "rejects malformed json payload",
			raw: []byte(
				"data: {\"id\":\"chatcmpl-1\"\n\n" +
					"data: [DONE]\n\n",
			),
			want: false,
		},
		{
			name: "rejects comment only stream",
			raw: []byte(
				": keep-alive\n\n" +
					": still-alive\n\n" +
					"data: [DONE]\n\n",
			),
			want: false,
		},
		{
			name: "rejects payload after done",
			raw: []byte(
				"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp-1\"}}\n\n" +
					"data: [DONE]\n\n" +
					"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-1\"}}\n\n",
			),
			want: false,
		},
		{
			name: "rejects trailing partial event",
			raw: []byte(
				"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\n" +
					"data: [DONE]\n\n" +
					": trailing keepalive",
			),
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := validateCacheableSSE(tt.raw); got != tt.want {
				t.Fatalf("validateCacheableSSE() = %v, want %v", got, tt.want)
			}
		})
	}
}
