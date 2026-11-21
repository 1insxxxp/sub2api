// Package wireusage projects provider counters onto customer-visible wire usage.
// A Projector belongs to one logical response. Its small counter snapshot lets
// a later cumulative usage event correct fields already sent in earlier events.
package wireusage

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type CapFunc func(read, input, shortWrite, longWrite int) (int, int, int, int)

type Projector struct {
	// ProjectStreamUsage is evaluated at the start of each wire usage object.
	// A nil predicate projects all usage. A false result streams the raw object.
	ProjectStreamUsage                     func() bool
	cap                                    CapFunc
	messages                               bool
	gross, input, read, write, short, long int
	hasInput, hasGross, hasRead, hasWrite  bool
	emitted, modalities                    map[string]int
	stream                                 jsonStream
	sse                                    sseStream
	cacheDetails, promptDetails            modalitySnapshot
}

func New(cap CapFunc, messages bool) *Projector {
	return &Projector{cap: cap, messages: messages, emitted: make(map[string]int), modalities: make(map[string]int)}
}

// JSON projects a complete JSON frame. The caller owns the original storage.
func (p *Projector) JSON(raw []byte) []byte {
	if !bytes.Contains(raw, []byte("usage")) || !json.Valid(raw) {
		return raw
	}
	root := gjson.ParseBytes(raw)
	out := raw
	if root.IsArray() {
		var frames bytes.Buffer
		// Process in one pass: a native array is a sequence of cumulative frames.
		_, _ = p.WriteJSON(&frames, raw)
		return frames.Bytes()
	}
	for _, path := range []string{"usage", "message.usage", "response.usage", "usageMetadata", "response.usageMetadata"} {
		obj := root.Get(path)
		if !obj.IsObject() {
			continue
		}
		src := []byte(obj.Raw)
		msg := p.messages || path == "message.usage" || root.Get("type").String() == "message" || root.Get("type").String() == "message_delta"
		dst := p.object(src, msg, path == "usageMetadata" || path == "response.usageMetadata")
		if !bytes.Equal(src, dst) {
			out, _ = sjson.SetRawBytes(out, path, dst)
		}
	}
	return out
}

var readFields = []string{"input_tokens_details.cached_tokens", "prompt_tokens_details.cached_tokens", "cache_read_input_tokens", "cache_read_tokens", "cached_tokens", "prompt_cache_hit_tokens"}
var writeFields = []string{"input_tokens_details.cache_write_tokens", "prompt_tokens_details.cache_write_tokens", "input_tokens_details.cache_creation_tokens", "prompt_tokens_details.cache_creation_tokens", "cache_write_tokens", "cache_creation_input_tokens", "cache_write_input_tokens", "cache_creation_tokens"}
var shortFields = []string{"cache_creation.ephemeral_5m_input_tokens", "input_tokens_details.cache_write_tokens_details.ttl_30m_tokens", "prompt_tokens_details.cache_write_tokens_details.ttl_30m_tokens"}
var longFields = []string{"cache_creation.ephemeral_1h_input_tokens", "input_tokens_details.cache_write_tokens_details.ttl_12h_tokens", "prompt_tokens_details.cache_write_tokens_details.ttl_12h_tokens"}

type usageObject struct {
	raw []byte
	src gjson.Result
	p   *Projector
}

// Match gateway token-counter bounds. An invalid recognized count provides no
// verified customer input. Null still means unreported.
func count(v gjson.Result) (int, bool) {
	if !v.Exists() || v.Type == gjson.Null {
		return 0, false
	}
	n, err := strconv.ParseInt(v.Raw, 10, 32)
	if err != nil || n < 0 {
		return 0, true
	}
	return int(n), true
}
func (u *usageObject) first(fields ...string) (int, bool) {
	for _, f := range fields {
		if n, ok := count(u.src.Get(f)); ok {
			return n, true
		}
	}
	return 0, false
}
func (u *usageObject) observe(dst *int, fields ...string) bool {
	if n, ok := u.first(fields...); ok {
		*dst = n
		return true
	}
	return false
}

// Emit a previously seen field again if a late cache measurement changed its
// allocation. Omitted and explicit-zero fields remain distinguishable.
func (u *usageObject) set(n int, fields ...string) {
	for _, f := range fields {
		v := u.src.Get(f)
		old, seen := u.p.emitted[f]
		if !v.Exists() || v.Type == gjson.Null {
			if !seen || old == n {
				continue
			}
		}
		u.p.emitted[f] = n
		if v.Type == gjson.Number && v.Raw == strconv.Itoa(n) {
			continue
		}
		u.raw, _ = sjson.SetBytes(u.raw, f, n)
	}
}

// Provider-reported prices cannot be prorated from token counts. A native
// monetary total would describe a different input when the cap binds.
func (u *usageObject) finish(discounted bool) []byte {
	if discounted {
		for _, key := range []string{"cost", "cost_details", "cost_in_usd_ticks", "input_cost", "total_cost"} {
			if u.src.Get(key).Exists() {
				u.raw, _ = sjson.DeleteBytes(u.raw, key)
			}
		}
	}
	return u.raw
}

func (u *usageObject) total(field string, old, new int, outputFields ...string) {
	total, ok := u.first(field)
	if !ok {
		return
	}
	output, _ := u.first(outputFields...)
	// Keep independently reported output (e.g. reasoning or Gemini tool tokens).
	n := max(output+new, total-old+new)
	if !u.p.hasGross && !u.p.hasInput {
		// A lone total cannot distinguish unverified input from output.
		n = output + new
		if field == "totalTokenCount" {
			thoughts, _ := u.first("thoughtsTokenCount")
			n += thoughts
		}
	}
	u.set(n, field)
}

func (p *Projector) object(raw []byte, messages, gemini bool) []byte {
	if !json.Valid(raw) {
		return raw
	}
	u := usageObject{raw: raw, src: gjson.ParseBytes(raw), p: p}
	if gemini {
		if u.observe(&p.gross, "promptTokenCount") {
			p.hasGross = true
		}
		u.observe(&p.read, "cachedContentTokenCount")
		if !p.hasGross {
			return raw
		}
		read := p.read
		if p.hasGross {
			read = min(read, p.gross)
		}
		r, i, _, _ := p.cap(read, max(0, p.gross-read), 0, 0)
		u.set(r+i, "promptTokenCount")
		u.set(r, "cachedContentTokenCount")
		u.total("totalTokenCount", p.gross, r+i, "candidatesTokenCount")
		u.geminiDetails(r, i)
		return u.finish(r+i < p.gross || r < p.read)
	}
	if messages {
		if u.observe(&p.input, "input_tokens") {
			p.hasInput = true
		}
		if u.observe(&p.gross, "prompt_tokens") {
			p.hasGross = true
		}
	} else if u.observe(&p.gross, "input_tokens", "prompt_tokens") {
		p.hasGross = true
	}
	if u.observe(&p.read, readFields...) {
		p.hasRead = true
	}
	if !p.hasRead {
		// Some OpenAI-compatible sources report only cached modality counters.
		for _, prefix := range []string{"input_tokens_details.", "prompt_tokens_details."} {
			detail := u.src.Get(prefix + "cached_tokens_details")
			if detail.IsObject() {
				p.read = 0
				for _, field := range []string{"image_tokens", "text_tokens", "audio_tokens", "video_tokens"} {
					n, _ := count(detail.Get(field))
					p.read += n
				}
				break
			}
		}
	}
	if u.observe(&p.write, writeFields...) {
		p.hasWrite = true
	}
	u.observe(&p.short, shortFields...)
	u.observe(&p.long, longFields...)
	writes := p.write
	if !p.hasWrite || p.write == 0 && messages {
		writes = p.short + p.long
	}
	read, input := p.read, p.input
	if p.hasGross {
		read = min(read, p.gross)
		writes = min(writes, p.gross-read)
		input = p.gross - read - writes
	}
	if miss, ok := u.first("prompt_cache_miss_tokens"); ok && messages {
		input = miss
		p.input = miss
		p.hasInput = true
	}
	if !p.hasGross && !p.hasInput {
		return raw
	}
	// Attribute untyped writes to the short bucket for cap allocation; expose
	// only measured TTL detail. Reconcile contradictory detail with the aggregate.
	short, long := p.short, p.long
	if short+long > writes {
		short = int(math.Round(float64(writes) * float64(short) / float64(short+long)))
		long = writes - short
	}
	r, i, s, l := p.cap(read, input, writes-long, long)
	gross := r + i + s + l
	if messages {
		u.set(i, "input_tokens")
		u.set(gross, "prompt_tokens")
	} else {
		u.set(gross, "input_tokens", "prompt_tokens")
	}
	u.set(r, readFields...)
	u.set(s+l, writeFields...)
	u.set(min(s, short), shortFields...)
	u.set(l, longFields...)
	u.set(i, "prompt_cache_miss_tokens")
	before := p.gross
	if !p.hasGross {
		before = input + read + writes
	}
	u.total("total_tokens", before, gross, "output_tokens", "completion_tokens")
	for _, prefix := range []string{"input_tokens_details.", "prompt_tokens_details."} {
		u.openAIDetails(prefix, r, i, s+l)
	}
	return u.finish(gross < before || r < p.read || s+l < writes)
}

// Reserve cached modalities within each prompt breakdown before distributing
// the remainder. Each displayed detail is bounded by its own original count.
func (u *usageObject) openAIDetails(prefix string, read, input, writes int) {
	budget := read
	fields := [...]string{"image_tokens", "audio_tokens", "text_tokens", "video_tokens"}
	var cached [len(fields)]int
	for i, field := range fields {
		path := prefix + "cached_tokens_details." + field
		if n, ok := u.first(path); ok {
			u.p.modalities[path] = n
		}
		cached[i] = min(u.p.modalities[path], budget)
		budget -= cached[i]
		u.set(cached[i], path)
	}
	regular := input + writes + budget
	for i, field := range fields {
		path := prefix + field
		if n, ok := u.first(path); ok {
			u.p.modalities[path] = n
		}
		n := u.p.modalities[path]
		covered := min(n, cached[i])
		plain := min(n-covered, regular)
		regular -= plain
		u.set(covered+plain, path)
	}
}

type modalitySnapshot struct {
	raw, projected string
}

// Track the two small arrays independently: native Gemini may send the cache
// breakdown after the prompt breakdown. Retain only these bounded fields.
func (u *usageObject) modalitiesFor(path string, snapshot *modalitySnapshot, allocate func(string, int) int) {
	field := u.src.Get(path)
	raw := snapshot.raw
	if field.IsArray() {
		raw = field.Raw
	}
	if raw == "" {
		return
	}
	var out strings.Builder
	last := 0
	gjson.Parse(raw).ForEach(func(_, v gjson.Result) bool {
		token := v.Get("tokenCount")
		n, ok := count(token)
		if !ok {
			return true
		}
		keep := strconv.Itoa(allocate(v.Get("modality").String(), n))
		if keep != token.Raw {
			_, _ = out.WriteString(raw[last:token.Index])
			_, _ = out.WriteString(keep)
			last = token.Index + len(token.Raw)
		}
		return true
	})
	projected := raw
	if last != 0 {
		_, _ = out.WriteString(raw[last:])
		projected = out.String()
	}
	if field.IsArray() && projected != raw || !field.IsArray() && projected != snapshot.projected {
		u.raw, _ = sjson.SetRawBytes(u.raw, path, []byte(projected))
	}
	if field.IsArray() {
		if len(raw) > maxFragmentedUsage {
			*snapshot = modalitySnapshot{}
			return
		}
		snapshot.raw = strings.Clone(raw)
	}
	if projected == raw {
		snapshot.projected = snapshot.raw
	} else {
		snapshot.projected = projected
	}
}

func (u *usageObject) geminiDetails(read, input int) {
	cached := map[string]int{}
	u.modalitiesFor("cacheTokensDetails", &u.p.cacheDetails, func(modality string, n int) int {
		kept := min(read, n)
		read -= kept
		cached[modality] += kept
		return kept
	})
	rest := input + read
	u.modalitiesFor("promptTokensDetails", &u.p.promptDetails, func(modality string, n int) int {
		kept := min(cached[modality], n)
		cached[modality] -= kept
		extra := min(rest, n-kept)
		rest -= extra
		return kept + extra
	})
}
