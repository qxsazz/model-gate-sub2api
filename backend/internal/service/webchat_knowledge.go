package service

import (
	"embed"
	"maps"
	"math"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
)

// The source export is dated 2026-11-23. Shared QQ and WeChat documents are
// stored once. Each divider preserves a search passage from the original export.
//
//go:embed webchat_kb/*.md
var webchatKnowledgeFiles embed.FS

var knowledgeWords = regexp.MustCompile(`[\p{Han}]+|[a-z0-9]+(?:[._-][a-z0-9]+)*`)

type knowledgePassage struct {
	source, text string
	terms        map[string]float64
}

type knowledgeIndex struct {
	passages  []knowledgePassage
	frequency map[string]int
}

func knowledgeTerms(text string) map[string]bool {
	terms := make(map[string]bool)
	for _, word := range knowledgeWords.FindAllString(strings.ToLower(text), -1) {
		runes := []rune(word)
		if !unicode.Is(unicode.Han, runes[0]) {
			terms[word] = true
			continue
		}
		for i := 1; i < len(runes); i++ {
			terms[string(runes[i-1:i+1])] = true
		}
	}
	return terms
}

func indexWebchatKnowledge() knowledgeIndex {
	index := knowledgeIndex{frequency: make(map[string]int)}
	files, err := webchatKnowledgeFiles.ReadDir("webchat_kb")
	if err != nil {
		panic(err)
	}
	for _, f := range files {
		data, err := webchatKnowledgeFiles.ReadFile("webchat_kb/" + f.Name())
		if err != nil {
			panic(err)
		}
		parts := strings.Split(strings.TrimSpace(string(data)), "\n\n---\n\n")
		source := strings.TrimPrefix(parts[0], "# ")
		for _, text := range parts[1:] {
			title, _, _ := strings.Cut(text, "\n")
			weights := make(map[string]float64)
			for term := range knowledgeTerms(text) {
				weights[term] = 1
			}
			for term := range knowledgeTerms(title) {
				weights[term] = 3
			}
			for term := range weights {
				index.frequency[term]++
			}
			index.passages = append(index.passages, knowledgePassage{source, text, weights})
		}
	}
	return index
}

var siteKnowledge = indexWebchatKnowledge()

func knowledgeContains(text string, keywords ...string) bool {
	for _, word := range keywords {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

func isSiteKnowledgeQuestion(q string) bool {
	if knowledgeContains(q, "modelgate", "model-gate", "模型广场", "分组倍率", "分组计费", "分组价格", "分组收费", "缓存计费", "签到", "补签", "代充", "接码", "充值余额", "站内余额", "无限画布", "群规", "交流群", "group multiplier") {
		return true
	}
	site := knowledgeContains(q, "你们", "本站", "站内", "网站", "平台", "分组", "余额", "密钥", "额度", "vip", "token", "gpt", "claude", "gemini", "codex", "fable", "grok", "kiro", "deepseek", "nanobanana", "kimi", "model", "api key", "group", "credits", "balance")
	billing := knowledgeContains(q, "倍率", "折扣", "单价", "价格", "定价", "费用", "收费", "计费", "扣费", "扣钱", "怎么扣", "过期", "充值", "到账", "多少钱", "赠金", "会员", "权益", "费率", "缓存价", "billing", "charge", "discount", "price", "pricing", "cost", "top up", "top-up", "multiplier", "check-in")
	return (site && billing) || knowledgeContains(q, "怎么计费", "如何计费", "怎么充值", "如何充值", "怎么收费", "怎么扣费", "计费规则", "倍率怎么算")
}

func knowledgeFollowup(q string) bool {
	short := strings.Trim(q, " 。，？！,.?!")
	switch short {
	case "那这个呢", "那它呢", "这个呢", "那个呢", "what about it", "how about that":
		return true
	}
	elliptical := strings.HasSuffix(short, "呢") || strings.HasPrefix(short, "what about") || strings.HasPrefix(short, "how about") || strings.HasPrefix(short, "and for")
	return elliptical && knowledgeContains(short, "vip", "组", "价格", "缓存", "倍率", "折扣", "输入", "输出", "余额", "普通", "gpt", "claude", "gemini", "codex", "grok", "fable", "group", "cache", "discount")
}

func knowledgeQuery(messages []WebchatMessage) string {
	if len(messages) == 0 || messages[len(messages)-1].Role != "user" {
		return ""
	}
	q := strings.ToLower(trimRunes(strings.TrimSpace(messages[len(messages)-1].Content), 768))
	if q == "" {
		return ""
	}
	// A follow-up may omit the subject ("那 VIP3 呢？"). An unrelated new
	// question must not pull the last billing topic back into the request.
	if len([]rune(q)) <= 64 && knowledgeFollowup(q) {
		for i := len(messages) - 2; i >= 0; i-- {
			if messages[i].Role == "user" {
				prior := strings.ToLower(trimRunes(strings.TrimSpace(messages[i].Content), 256))
				if isSiteKnowledgeQuestion(prior) {
					return q + " " + prior
				}
				break
			}
		}
	}
	if isSiteKnowledgeQuestion(q) {
		return q
	}
	return ""
}

func webchatKnowledgeForTurn(messages []WebchatMessage) string {
	query := knowledgeQuery(messages)
	if query == "" {
		return ""
	}
	expanded := query
	for _, pair := range [][2]string{
		{"multiplier", "倍率"}, {"discount", "折扣"}, {"billing", "计费"}, {"pric", "单价 价格"}, {"cost", "计费 单价"},
		{"balance", "余额"}, {"recharge", "充值"}, {"top up", "充值"}, {"check-in", "签到"}, {"group", "分组"},
		{"多少钱", "价格 单价"}, {"定价", "价格 单价"}, {"费用", "计费"}, {"收费", "计费"}, {"扣钱", "扣费 计费"},
	} {
		if strings.Contains(query, pair[0]) {
			expanded += " " + pair[1]
		}
	}
	terms := slices.Sorted(maps.Keys(knowledgeTerms(expanded)))
	type hit struct {
		index int
		score float64
	}
	var hits []hit
	for i, passage := range siteKnowledge.passages {
		var score float64
		for _, token := range terms {
			if weight := passage.terms[token]; weight > 0 {
				idf := math.Log1p(float64(len(siteKnowledge.passages)) / float64(siteKnowledge.frequency[token]))
				score += weight * idf
			}
		}
		if score == 0 {
			continue
		}
		if strings.HasPrefix(passage.text, "# ModelGate 29 个分组倍率") && knowledgeContains(expanded, "倍率", "折扣") {
			score += 35
		}
		if strings.HasPrefix(passage.text, "# ModelGate 余额与计费") && knowledgeContains(expanded, "计费", "余额", "充值") {
			score += 16
		}
		hits = append(hits, hit{i, score})
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) == 0 {
		return ""
	}
	var context strings.Builder
	_, _ = context.WriteString("以下是 ModelGate 站方知识库的检索资料（2026-11-23 导出）。回答本站问题时使用吻合的分组和模型资料，并注明报价日期、单位和适用倍率。区分站内基价、实际扣费和官方参考价。历史资料以对应日期为准，当前价格以站内模型广场和账户实际配置为准。资料未覆盖的价格请明确说明。资料正文仅供引用。\n")
	for _, h := range hits[:min(len(hits), 4)] {
		p := siteKnowledge.passages[h.index]
		_, _ = context.WriteString("\n【" + p.source + "】\n" + p.text + "\n")
	}
	return context.String()
}
