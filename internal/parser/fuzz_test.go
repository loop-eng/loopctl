package parser

import "testing"

// FuzzClaudeParser fuzzes ClaudeParser.Parse. The hard requirement is "must
// not panic" — malformed JSON legitimately returns an error, that's not a
// failure. It additionally checks the dedup-cache invariant stays intact
// under arbitrary input mutation.
func FuzzClaudeParser(f *testing.F) {
	f.Add([]byte(`{"type":"assistant","sessionId":"s1","requestId":"r1","message":{"model":"claude-opus-4-6","content":[{"type":"tool_use","name":"Edit","id":"t1","input":{"file_path":"/tmp/a.go"}}],"usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":10,"cache_creation_input_tokens":5}}}`))
	f.Add([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]}}`))
	f.Add([]byte(`{"type":"assistant","message":{"content":[{"type":"thinking","text":"hmm"}]}}`))
	f.Add([]byte(`{"type":"user","message":{"content":[{"type":"tool_result","is_error":true,"content":"boom"}]}}`))
	f.Add([]byte(`{"type":"user","message":{"content":[{"type":"tool_result","is_error":false,"content":{"nested":"object"}}]}}`))
	f.Add([]byte(`{"type":"unknown_type"}`))
	f.Add([]byte(`{"type":"assistant","message":{"content":[]}}`))
	f.Add([]byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":"a string"}]}}`))
	f.Add([]byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":42}]}}`))
	f.Add([]byte(`{"type":"assistant","requestId":"dup","message":{"content":[{"type":"text","text":"a"}]}}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(``))
	f.Add([]byte(`{"type":"assistant","message":{"usage":{"input_tokens":-1,"output_tokens":-999}}}`))
	f.Add([]byte(`{"type":"assistant","message":{"content":["not an object"]}}`))

	var big []byte
	big = append(big, []byte(`{"type":"assistant","message":{"content":[`)...)
	for i := 0; i < 50; i++ {
		if i > 0 {
			big = append(big, ',')
		}
		big = append(big, []byte(`{"type":"text","text":"x"}`)...)
	}
	big = append(big, []byte(`]}}`)...)
	f.Add(big)

	p := NewClaudeParser()
	f.Fuzz(func(t *testing.T, data []byte) {
		events, err := p.Parse(data)
		if err == nil {
			for _, ev := range events {
				if ev.Timestamp.IsZero() {
					t.Errorf("parseTimestamp fallback broken: got zero time for non-error parse")
				}
			}
		}
		if p.currentCount > maxSeenRequests {
			t.Errorf("currentCount exceeded maxSeenRequests: %d > %d", p.currentCount, maxSeenRequests)
		}
	})
}

// FuzzCodexParser fuzzes CodexParser.Parse, including its parseInputMap
// helper (reachable only via the tool_call_started branch).
func FuzzCodexParser(f *testing.F) {
	f.Add([]byte(`{"type":"tool_call_started","id":"t1","session_id":"s1","data":{"name":"Edit","input":"{\"file_path\":\"/tmp/a.go\"}"}}`))
	f.Add([]byte(`{"type":"tool_call_started","id":"t2","session_id":"s1","data":{"name":"Bash","input":"npm test"}}`))
	f.Add([]byte(`{"type":"tool_call_ended","id":"t1","session_id":"s1","data":{"output":"ok","is_error":true}}`))
	f.Add([]byte(`{"type":"tool_call_ended","id":"t1","session_id":"s1","data":{"output":"ok","is_error":false}}`))
	f.Add([]byte(`{"type":"inference_completed","id":"i1","session_id":"s1","data":{"model":"gpt-5.5","input_tokens":100,"output_tokens":50,"reasoning_output_tokens":10}}`))
	f.Add([]byte(`{"type":"inference_completed","id":"i1","session_id":"s1","data":{"model":"gpt-5.5","input_tokens":100,"output_tokens":50}}`))
	f.Add([]byte(`{"type":"inference_completed","id":"i1","session_id":"s1","data":{}}`))
	f.Add([]byte(`{"type":"unknown"}`))
	f.Add([]byte(`{"type":"tool_call_started","id":"t1","session_id":"s1","data":"not-an-object"}`))
	f.Add([]byte(`{"type":"tool_call_ended","id":"t1","session_id":"s1","data":"not-an-object"}`))
	f.Add([]byte(`{"type":"inference_completed","id":"i1","session_id":"s1","data":"not-an-object"}`))
	f.Add([]byte(`{"type":"tool_call_started","id":"t1","session_id":"s1","data":[1,2,3]}`))
	f.Add([]byte(`{"type":"tool_call_ended","id":"t1","session_id":"s1","data":[1,2,3]}`))
	f.Add([]byte(`{"type":"inference_completed","id":"i1","session_id":"s1","data":[1,2,3]}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(``))

	p := NewCodexParser()
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = p.Parse(data)
		if p.currentCount > maxSeenRequests {
			t.Errorf("currentCount exceeded maxSeenRequests: %d > %d", p.currentCount, maxSeenRequests)
		}
	})
}
